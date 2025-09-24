/*
Copyright (c) 2024 Ansible Project

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/work-obs/ansible-go/internal/router"
	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/work-obs/ansible-go/pkg/plugins"
)

// TaskStatus represents the status of a task execution
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusSkipped   TaskStatus = "skipped"
)

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID    string                 `json:"task_id"`
	Host      string                 `json:"host"`
	Status    TaskStatus             `json:"status"`
	Changed   bool                   `json:"changed"`
	Failed    bool                   `json:"failed"`
	Message   string                 `json:"msg,omitempty"`
	Result    map[string]interface{} `json:"result,omitempty"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
	Duration  time.Duration          `json:"duration"`
	Error     string                 `json:"error,omitempty"`
}

// Task represents a single task to be executed
type Task struct {
	ID           string                 `json:"id"`
	Name         string                 `json:"name"`
	Module       string                 `json:"module"`
	Args         map[string]interface{} `json:"args,omitempty"`
	Host         string                 `json:"host"`
	Vars         map[string]interface{} `json:"vars,omitempty"`
	When         string                 `json:"when,omitempty"`
	Loop         interface{}            `json:"loop,omitempty"`
	Delegate     string                 `json:"delegate_to,omitempty"`
	RunOnce      bool                   `json:"run_once,omitempty"`
	Async        int                    `json:"async,omitempty"`
	Poll         int                    `json:"poll,omitempty"`
	Timeout      time.Duration          `json:"timeout,omitempty"`
	Retries      int                    `json:"retries,omitempty"`
	Delay        time.Duration          `json:"delay,omitempty"`
	IgnoreErrors bool                   `json:"ignore_errors,omitempty"`
	ChangedWhen  string                 `json:"changed_when,omitempty"`
	FailedWhen   string                 `json:"failed_when,omitempty"`
	Tags         []string               `json:"tags,omitempty"`
}

// ExecutionContext holds the context for task execution
type ExecutionContext struct {
	Config         *config.Config
	Variables      map[string]interface{}
	Facts          map[string]interface{}
	HostVars       map[string]map[string]interface{}
	GroupVars      map[string]map[string]interface{}
	ExtraVars      map[string]interface{}
	Inventory      map[string]interface{}
	ConnectionInfo map[string]interface{}
}

// Executor manages task execution
type Executor struct {
	config     *config.Config
	router     *router.Router
	pluginMgr  plugins.Manager
	results    map[string]*TaskResult
	mutex      sync.RWMutex
	maxWorkers int
	taskQueue  chan *Task
	resultChan chan *TaskResult
	workerPool chan chan *Task
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewExecutor creates a new task executor
func NewExecutor(cfg *config.Config, router *router.Router, pluginMgr plugins.Manager) *Executor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Executor{
		config:     cfg,
		router:     router,
		pluginMgr:  pluginMgr,
		results:    make(map[string]*TaskResult),
		maxWorkers: 5, // Default to 5 workers
		taskQueue:  make(chan *Task, 100),
		resultChan: make(chan *TaskResult, 100),
		workerPool: make(chan chan *Task, 5),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the executor workers
func (e *Executor) Start() error {
	// Start worker pool
	for i := 0; i < e.maxWorkers; i++ {
		worker := &TaskWorker{
			ID:         i,
			TaskChan:   make(chan *Task),
			ResultChan: e.resultChan,
			WorkerPool: e.workerPool,
			Executor:   e,
			ctx:        e.ctx,
		}
		go worker.Start()
	}

	// Start dispatcher
	go e.dispatch()

	// Start result collector
	go e.collectResults()

	return nil
}

// Stop stops the executor
func (e *Executor) Stop() {
	e.cancel()
	close(e.taskQueue)
}

// ExecuteTask executes a single task
func (e *Executor) ExecuteTask(task *Task, execCtx *ExecutionContext) (*TaskResult, error) {
	result := &TaskResult{
		TaskID:    task.ID,
		Host:      task.Host,
		Status:    TaskStatusPending,
		StartTime: time.Now(),
	}

	// Store result for tracking
	e.mutex.Lock()
	e.results[task.ID] = result
	e.mutex.Unlock()

	// Resolve module name through router
	resolvedModule, err := e.router.ResolveModule(task.Module)
	if err != nil {
		return e.failTask(result, fmt.Sprintf("Failed to resolve module '%s': %v", task.Module, err))
	}

	// Check if module is deprecated
	if deprecated, warning := e.router.IsModuleDeprecated(resolvedModule); deprecated {
		// Log deprecation warning but continue execution
		result.Message = warning
	}

	// Load the module plugin
	plugin, err := e.pluginMgr.LoadModule(resolvedModule)
	if err != nil {
		return e.failTask(result, fmt.Sprintf("Failed to load module '%s': %v", resolvedModule, err))
	}

	// Check conditional execution
	if task.When != "" {
		shouldRun, err := e.evaluateCondition(task.When, execCtx)
		if err != nil {
			return e.failTask(result, fmt.Sprintf("Failed to evaluate 'when' condition: %v", err))
		}
		if !shouldRun {
			result.Status = TaskStatusSkipped
			result.EndTime = time.Now()
			result.Duration = result.EndTime.Sub(result.StartTime)
			return result, nil
		}
	}

	// Handle retries
	maxRetries := task.Retries
	if maxRetries == 0 {
		maxRetries = 1 // At least one attempt
	}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Add delay between retries
			if task.Delay > 0 {
				time.Sleep(task.Delay)
			} else {
				time.Sleep(time.Second) // Default 1 second delay
			}
		}

		result.Status = TaskStatusRunning

		// Execute the module
		moduleResult, err := e.executeModule(plugin, task, execCtx)
		if err != nil {
			lastErr = err
			if attempt == maxRetries-1 { // Last attempt
				return e.failTask(result, fmt.Sprintf("Task failed after %d attempts: %v", maxRetries, err))
			}
			continue
		}

		// Process module result
		result.Result = moduleResult
		result.Status = TaskStatusCompleted
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)

		// Check if task changed something
		if changed, ok := moduleResult["changed"].(bool); ok {
			result.Changed = changed
		}

		// Evaluate custom changed_when condition
		if task.ChangedWhen != "" {
			changed, err := e.evaluateCondition(task.ChangedWhen, execCtx)
			if err != nil {
				return e.failTask(result, fmt.Sprintf("Failed to evaluate 'changed_when' condition: %v", err))
			}
			result.Changed = changed
		}

		// Evaluate custom failed_when condition
		if task.FailedWhen != "" {
			failed, err := e.evaluateCondition(task.FailedWhen, execCtx)
			if err != nil {
				return e.failTask(result, fmt.Sprintf("Failed to evaluate 'failed_when' condition: %v", err))
			}
			if failed {
				return e.failTask(result, "Task failed due to 'failed_when' condition")
			}
		}

		// Check if module reported failure
		if failed, ok := moduleResult["failed"].(bool); ok && failed {
			if !task.IgnoreErrors {
				msg := "Module execution failed"
				if errMsg, ok := moduleResult["msg"].(string); ok {
					msg = errMsg
				}
				return e.failTask(result, msg)
			}
			result.Failed = true
		}

		return result, nil
	}

	return e.failTask(result, fmt.Sprintf("Task failed after %d attempts: %v", maxRetries, lastErr))
}

// QueueTask queues a task for execution
func (e *Executor) QueueTask(task *Task) {
	select {
	case e.taskQueue <- task:
	case <-e.ctx.Done():
		// Executor is shutting down
	}
}

// GetResult returns the result of a task
func (e *Executor) GetResult(taskID string) (*TaskResult, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()
	result, exists := e.results[taskID]
	return result, exists
}

// GetAllResults returns all task results
func (e *Executor) GetAllResults() map[string]*TaskResult {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	results := make(map[string]*TaskResult)
	for k, v := range e.results {
		results[k] = v
	}
	return results
}

// failTask marks a task as failed
func (e *Executor) failTask(result *TaskResult, errorMsg string) (*TaskResult, error) {
	result.Status = TaskStatusFailed
	result.Failed = true
	result.Error = errorMsg
	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	return result, fmt.Errorf("%s", errorMsg)
}

// dispatch dispatches tasks to workers
func (e *Executor) dispatch() {
	for {
		select {
		case task := <-e.taskQueue:
			select {
			case workerTaskChan := <-e.workerPool:
				workerTaskChan <- task
			case <-e.ctx.Done():
				return
			}
		case <-e.ctx.Done():
			return
		}
	}
}

// collectResults collects results from workers
func (e *Executor) collectResults() {
	for {
		select {
		case result := <-e.resultChan:
			e.mutex.Lock()
			e.results[result.TaskID] = result
			e.mutex.Unlock()
		case <-e.ctx.Done():
			return
		}
	}
}

// executeModule executes a module plugin
func (e *Executor) executeModule(plugin plugins.ExecutablePlugin, task *Task, execCtx *ExecutionContext) (map[string]interface{}, error) {
	// Create module context
	moduleCtx := &plugins.ModuleContext{
		Args:      task.Args,
		Variables: execCtx.Variables,
		Facts:     execCtx.Facts,
		Config:    execCtx.Config,
	}

	// Set timeout if specified
	ctx := e.ctx
	if task.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, task.Timeout)
		defer cancel()
	}

	// Execute the plugin
	return plugin.Execute(ctx, moduleCtx)
}

// evaluateCondition evaluates a conditional expression
func (e *Executor) evaluateCondition(condition string, execCtx *ExecutionContext) (bool, error) {
	// TODO: Implement proper condition evaluation using a template engine
	// For now, return true (always execute)
	// This should be replaced with proper Jinja2-like template evaluation
	return true, nil
}

// SetMaxWorkers sets the maximum number of worker goroutines
func (e *Executor) SetMaxWorkers(workers int) {
	e.maxWorkers = workers
}

// TaskWorker represents a worker that processes tasks
type TaskWorker struct {
	ID         int
	TaskChan   chan *Task
	ResultChan chan *TaskResult
	WorkerPool chan chan *Task
	Executor   *Executor
	ctx        context.Context
}

// Start starts the worker
func (w *TaskWorker) Start() {
	for {
		// Add worker to pool
		w.WorkerPool <- w.TaskChan

		select {
		case task := <-w.TaskChan:
			// Process task
			result, _ := w.processTask(task)
			w.ResultChan <- result
		case <-w.ctx.Done():
			return
		}
	}
}

// processTask processes a single task
func (w *TaskWorker) processTask(task *Task) (*TaskResult, error) {
	// Create execution context
	execCtx := &ExecutionContext{
		Config:    w.Executor.config,
		Variables: make(map[string]interface{}),
		Facts:     make(map[string]interface{}),
	}

	// Merge task variables
	if task.Vars != nil {
		for k, v := range task.Vars {
			execCtx.Variables[k] = v
		}
	}

	return w.Executor.ExecuteTask(task, execCtx)
}

// Legacy compatibility functions

// Config holds task executor configuration (legacy compatibility)
type Config struct {
	Forks      int
	Timeout    time.Duration
	Connection string
	User       string
	Become     bool
	BecomeUser string
	Check      bool
	Diff       bool
	Verbose    int
}

// TaskExecutor handles task execution (legacy compatibility)
type TaskExecutor struct {
	config        *Config
	ansibleConfig *config.Config
	executor      *Executor
}

// NewTaskExecutor creates a new task executor (legacy compatibility)
func NewTaskExecutor(config *Config, ansibleConfig *config.Config) (*TaskExecutor, error) {
	return &TaskExecutor{
		config:        config,
		ansibleConfig: ansibleConfig,
	}, nil
}

// ExecuteModule executes a module on the specified hosts (legacy compatibility)
func (e *TaskExecutor) ExecuteModule(ctx context.Context, hosts []string, moduleName string, args map[string]interface{}) (map[string]*TaskResult, error) {
	results := make(map[string]*TaskResult)

	// TODO: Implement actual module execution using new executor
	for _, host := range hosts {
		results[host] = &TaskResult{
			Status:    TaskStatusCompleted,
			Message:   "Module execution not yet fully implemented",
			Changed:   false,
			Failed:    false,
			Result:    make(map[string]interface{}),
			StartTime: time.Now(),
			EndTime:   time.Now(),
		}
	}

	return results, nil
}
