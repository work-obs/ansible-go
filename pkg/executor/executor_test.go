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
	"testing"
	"time"

	"github.com/ansible/ansible-go/pkg/config"
	"github.com/ansible/ansible-go/pkg/plugins"
	"github.com/ansible/ansible-go/internal/router"
	"github.com/spf13/afero"
)

// MockPlugin implements the Plugin interface for testing
type MockPlugin struct {
	name        string
	shouldFail  bool
	shouldError bool
	result      map[string]interface{}
}

func (m *MockPlugin) Name() string {
	return m.name
}

func (m *MockPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeModule
}

func (m *MockPlugin) Execute(ctx context.Context, moduleCtx *plugins.ModuleContext) (map[string]interface{}, error) {
	if m.shouldError {
		return nil, &plugins.PluginError{Message: "Mock plugin error"}
	}

	result := make(map[string]interface{})
	if m.result != nil {
		for k, v := range m.result {
			result[k] = v
		}
	} else {
		result["changed"] = !m.shouldFail
		result["failed"] = m.shouldFail
		result["msg"] = "Mock execution result"
	}

	return result, nil
}

func (m *MockPlugin) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        m.name,
		Type:        plugins.PluginTypeModule,
		Description: "Mock plugin for testing",
		Version:     "1.0.0",
	}
}

func (m *MockPlugin) Validate(args map[string]interface{}) error {
	return nil
}

// MockPluginManager implements the Manager interface for testing
type MockPluginManager struct {
	plugins map[string]plugins.Plugin
}

func NewMockPluginManager() *MockPluginManager {
	return &MockPluginManager{
		plugins: make(map[string]plugins.Plugin),
	}
}

func (m *MockPluginManager) AddPlugin(name string, plugin plugins.Plugin) {
	m.plugins[name] = plugin
}

func (m *MockPluginManager) LoadModule(name string) (plugins.Plugin, error) {
	if plugin, exists := m.plugins[name]; exists {
		return plugin, nil
	}
	return nil, &plugins.PluginError{Message: "Plugin not found: " + name}
}

func (m *MockPluginManager) LoadPlugin(pluginType plugins.PluginType, name string) (plugins.Plugin, error) {
	return m.LoadModule(name)
}

func (m *MockPluginManager) GetAvailablePlugins(pluginType plugins.PluginType) ([]string, error) {
	var names []string
	for name := range m.plugins {
		names = append(names, name)
	}
	return names, nil
}

func (m *MockPluginManager) ValidatePlugin(pluginType plugins.PluginType, name string, args map[string]interface{}) error {
	return nil
}

func TestNewExecutor(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	executor := NewExecutor(cfg, router, pluginMgr)

	if executor == nil {
		t.Fatal("Expected non-nil executor")
	}

	if executor.config != cfg {
		t.Error("Expected config to be set correctly")
	}

	if executor.router != router {
		t.Error("Expected router to be set correctly")
	}

	if executor.pluginMgr != pluginMgr {
		t.Error("Expected plugin manager to be set correctly")
	}

	if executor.maxWorkers != 5 {
		t.Errorf("Expected default max workers to be 5, got %d", executor.maxWorkers)
	}
}

func TestExecutor_ExecuteTask_Success(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	// Add a mock plugin
	mockPlugin := &MockPlugin{
		name:       "test_module",
		shouldFail: false,
		result: map[string]interface{}{
			"changed": true,
			"msg":     "Test execution successful",
		},
	}
	pluginMgr.AddPlugin("test_module", mockPlugin)

	executor := NewExecutor(cfg, router, pluginMgr)

	task := &Task{
		ID:     "task-1",
		Name:   "Test Task",
		Module: "test_module",
		Host:   "test-host",
		Args: map[string]interface{}{
			"arg1": "value1",
		},
	}

	execCtx := &ExecutionContext{
		Config:    cfg,
		Variables: make(map[string]interface{}),
		Facts:     make(map[string]interface{}),
	}

	result, err := executor.ExecuteTask(task, execCtx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.Status != TaskStatusCompleted {
		t.Errorf("Expected status %s, got %s", TaskStatusCompleted, result.Status)
	}

	if !result.Changed {
		t.Error("Expected task to be marked as changed")
	}

	if result.Failed {
		t.Error("Expected task not to be marked as failed")
	}

	if result.TaskID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, result.TaskID)
	}

	if result.Host != task.Host {
		t.Errorf("Expected host %s, got %s", task.Host, result.Host)
	}
}

func TestExecutor_ExecuteTask_Failure(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	// Add a mock plugin that fails
	mockPlugin := &MockPlugin{
		name:       "test_module",
		shouldFail: true,
		result: map[string]interface{}{
			"changed": false,
			"failed":  true,
			"msg":     "Test execution failed",
		},
	}
	pluginMgr.AddPlugin("test_module", mockPlugin)

	executor := NewExecutor(cfg, router, pluginMgr)

	task := &Task{
		ID:     "task-1",
		Name:   "Test Task",
		Module: "test_module",
		Host:   "test-host",
	}

	execCtx := &ExecutionContext{
		Config:    cfg,
		Variables: make(map[string]interface{}),
		Facts:     make(map[string]interface{}),
	}

	result, err := executor.ExecuteTask(task, execCtx)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result.Status != TaskStatusFailed {
		t.Errorf("Expected status %s, got %s", TaskStatusFailed, result.Status)
	}

	if !result.Failed {
		t.Error("Expected task to be marked as failed")
	}
}

func TestExecutor_ExecuteTask_WithRetries(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	// Add a mock plugin that errors
	mockPlugin := &MockPlugin{
		name:        "test_module",
		shouldError: true,
	}
	pluginMgr.AddPlugin("test_module", mockPlugin)

	executor := NewExecutor(cfg, router, pluginMgr)

	task := &Task{
		ID:      "task-1",
		Name:    "Test Task",
		Module:  "test_module",
		Host:    "test-host",
		Retries: 3,
		Delay:   100 * time.Millisecond,
	}

	execCtx := &ExecutionContext{
		Config:    cfg,
		Variables: make(map[string]interface{}),
		Facts:     make(map[string]interface{}),
	}

	startTime := time.Now()
	result, err := executor.ExecuteTask(task, execCtx)
	duration := time.Since(startTime)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result.Status != TaskStatusFailed {
		t.Errorf("Expected status %s, got %s", TaskStatusFailed, result.Status)
	}

	// Should have taken at least the delay time * (retries - 1)
	expectedMinDuration := time.Duration(task.Retries-1) * task.Delay
	if duration < expectedMinDuration {
		t.Errorf("Expected duration at least %v, got %v", expectedMinDuration, duration)
	}
}

func TestExecutor_ExecuteTask_IgnoreErrors(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	// Add a mock plugin that fails
	mockPlugin := &MockPlugin{
		name:       "test_module",
		shouldFail: true,
		result: map[string]interface{}{
			"changed": false,
			"failed":  true,
			"msg":     "Test execution failed",
		},
	}
	pluginMgr.AddPlugin("test_module", mockPlugin)

	executor := NewExecutor(cfg, router, pluginMgr)

	task := &Task{
		ID:           "task-1",
		Name:         "Test Task",
		Module:       "test_module",
		Host:         "test-host",
		IgnoreErrors: true,
	}

	execCtx := &ExecutionContext{
		Config:    cfg,
		Variables: make(map[string]interface{}),
		Facts:     make(map[string]interface{}),
	}

	result, err := executor.ExecuteTask(task, execCtx)

	if err != nil {
		t.Fatalf("Expected no error when ignoring errors, got: %v", err)
	}

	if result.Status != TaskStatusCompleted {
		t.Errorf("Expected status %s, got %s", TaskStatusCompleted, result.Status)
	}

	if !result.Failed {
		t.Error("Expected task to be marked as failed even when ignoring errors")
	}
}

func TestExecutor_QueueTask(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	executor := NewExecutor(cfg, router, pluginMgr)

	task := &Task{
		ID:     "task-1",
		Name:   "Test Task",
		Module: "test_module",
		Host:   "test-host",
	}

	// Queue the task (should not block)
	executor.QueueTask(task)

	// Check that the task was queued
	select {
	case queuedTask := <-executor.taskQueue:
		if queuedTask.ID != task.ID {
			t.Errorf("Expected queued task ID %s, got %s", task.ID, queuedTask.ID)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Task was not queued within expected time")
	}
}

func TestExecutor_GetResult(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	executor := NewExecutor(cfg, router, pluginMgr)

	// Add a result directly to test retrieval
	result := &TaskResult{
		TaskID: "task-1",
		Host:   "test-host",
		Status: TaskStatusCompleted,
	}

	executor.mutex.Lock()
	executor.results["task-1"] = result
	executor.mutex.Unlock()

	// Test getting existing result
	retrievedResult, exists := executor.GetResult("task-1")
	if !exists {
		t.Fatal("Expected result to exist")
	}

	if retrievedResult.TaskID != result.TaskID {
		t.Errorf("Expected task ID %s, got %s", result.TaskID, retrievedResult.TaskID)
	}

	// Test getting non-existent result
	_, exists = executor.GetResult("nonexistent")
	if exists {
		t.Error("Expected result not to exist")
	}
}

func TestExecutor_SetMaxWorkers(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	executor := NewExecutor(cfg, router, pluginMgr)

	executor.SetMaxWorkers(10)

	if executor.maxWorkers != 10 {
		t.Errorf("Expected max workers to be 10, got %d", executor.maxWorkers)
	}
}

func TestTaskWorker_processTask(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	router := router.NewRouter()
	pluginMgr := NewMockPluginManager()

	// Add a mock plugin
	mockPlugin := &MockPlugin{
		name:       "test_module",
		shouldFail: false,
	}
	pluginMgr.AddPlugin("test_module", mockPlugin)

	executor := NewExecutor(cfg, router, pluginMgr)

	worker := &TaskWorker{
		ID:         0,
		TaskChan:   make(chan *Task),
		ResultChan: make(chan *TaskResult),
		WorkerPool: make(chan chan *Task),
		Executor:   executor,
		ctx:        executor.ctx,
	}

	task := &Task{
		ID:     "task-1",
		Name:   "Test Task",
		Module: "test_module",
		Host:   "test-host",
		Vars: map[string]interface{}{
			"test_var": "test_value",
		},
	}

	result, err := worker.processTask(task)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if result.TaskID != task.ID {
		t.Errorf("Expected task ID %s, got %s", task.ID, result.TaskID)
	}
}

// Legacy compatibility tests

func TestNewTaskExecutor(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	execConfig := &Config{
		Forks:   5,
		Timeout: 30 * time.Second,
	}

	executor, err := NewTaskExecutor(execConfig, cfg)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if executor == nil {
		t.Fatal("Expected non-nil executor")
	}

	if executor.config != execConfig {
		t.Error("Expected config to be set correctly")
	}

	if executor.ansibleConfig != cfg {
		t.Error("Expected ansible config to be set correctly")
	}
}

func TestTaskExecutor_ExecuteModule(t *testing.T) {
	fs := afero.NewMemMapFs()
	configMgr := config.NewManager(fs)
	err := configMgr.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}
	cfg := configMgr.GetConfig()

	execConfig := &Config{
		Forks:   5,
		Timeout: 30 * time.Second,
	}

	executor, err := NewTaskExecutor(execConfig, cfg)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	hosts := []string{"host1", "host2"}
	moduleName := "test_module"
	args := map[string]interface{}{
		"arg1": "value1",
	}

	ctx := context.Background()
	results, err := executor.ExecuteModule(ctx, hosts, moduleName, args)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(results) != len(hosts) {
		t.Errorf("Expected %d results, got %d", len(hosts), len(results))
	}

	for _, host := range hosts {
		result, exists := results[host]
		if !exists {
			t.Errorf("Expected result for host %s", host)
			continue
		}

		if result.Status != TaskStatusCompleted {
			t.Errorf("Expected status %s for host %s, got %s", TaskStatusCompleted, host, result.Status)
		}
	}
}