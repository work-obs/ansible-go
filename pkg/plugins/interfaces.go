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

package plugins

import (
	"context"
	"fmt"
	"time"
)

// PluginType represents the type of plugin
type PluginType string

const (
	PluginTypeAction       PluginType = "action"       // Module implementations
	PluginTypeBecome       PluginType = "become"       // Privilege escalation
	PluginTypeCache        PluginType = "cache"        // Fact/data caching
	PluginTypeCallback     PluginType = "callback"     // Event notifications
	PluginTypeCLIConf      PluginType = "cliconf"      // Network CLI configuration
	PluginTypeConnection   PluginType = "connection"   // Host connections
	PluginTypeFilter       PluginType = "filter"       // Jinja2 filters
	PluginTypeHTTPAPI      PluginType = "httpapi"      // HTTP API interactions
	PluginTypeInventory    PluginType = "inventory"    // Dynamic inventory
	PluginTypeLookup       PluginType = "lookup"       // Data lookups
	PluginTypeModule       PluginType = "module"       // Legacy module interface
	PluginTypeNetconf      PluginType = "netconf"      // NETCONF protocol
	PluginTypeShell        PluginType = "shell"        // Shell command formatting
	PluginTypeStrategy     PluginType = "strategy"     // Execution strategies
	PluginTypeTerminal     PluginType = "terminal"     // Terminal interactions
	PluginTypeTest         PluginType = "test"         // Jinja2 tests
	PluginTypeVars         PluginType = "vars"         // Variable sources
	PluginTypeDocFragments PluginType = "doc_fragments" // Documentation fragments
)


// PluginError represents an error from plugin execution
type PluginError struct {
	Message string
	Code    int
	Details map[string]interface{}
}

func (e *PluginError) Error() string {
	return e.Message
}

// ModuleContext provides context for module execution
type ModuleContext struct {
	Args      map[string]interface{}
	Variables map[string]interface{}
	Facts     map[string]interface{}
	Config    interface{}
}

// ActionContext provides context for action plugins
type ActionContext struct {
	ModuleContext
	TaskVars      map[string]interface{}
	PlayContext   *PlayContext
	Connection    interface{} // Connection plugin instance
	TempDir       string
	Loader        interface{} // Data loader instance
}

// ActionResult represents the result of an action plugin execution
type ActionResult struct {
	Changed   bool                   `json:"changed"`
	Failed    bool                   `json:"failed"`
	Skipped   bool                   `json:"skipped"`
	Message   string                 `json:"msg,omitempty"`
	Results   map[string]interface{} `json:"-"` // Merged into top level
	Warnings  []string               `json:"warnings,omitempty"`
	Diff      map[string]interface{} `json:"diff,omitempty"`
}

// PlayContext provides context for play execution
type PlayContext struct {
	PlayName    string                 `json:"play_name"`
	Hosts       []string               `json:"hosts"`
	Variables   map[string]interface{} `json:"variables"`
	Tags        []string               `json:"tags"`
	SkipTags    []string               `json:"skip_tags"`
	CheckMode   bool                   `json:"check_mode"`
	DiffMode    bool                   `json:"diff_mode"`
	Verbosity   int                    `json:"verbosity"`
	StartTime   time.Time              `json:"start_time"`
}

// PluginInfo contains metadata about a plugin
type PluginInfo struct {
	Name        string
	Type        PluginType
	Description string
	Version     string
	Author      []string
	Options     map[string]interface{}
}

// ExecutablePlugin interface that all executable plugins must implement
type ExecutablePlugin interface {
	Name() string
	Type() PluginType
	Execute(ctx context.Context, moduleCtx *ModuleContext) (map[string]interface{}, error)
	GetInfo() *PluginInfo
	Validate(args map[string]interface{}) error
}

// ModulePlugin interface for module plugins
type ModulePlugin interface {
	ExecutablePlugin
	Run(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

// ActionPlugin interface for action plugins
type ActionPlugin interface {
	BasePlugin
	Run(ctx context.Context, actionCtx *ActionContext) (*ActionResult, error)
	GetRequiredConnection() string
}

// BasePlugin interface that all plugins must implement
type BasePlugin interface {
	Name() string
	Type() PluginType
	GetInfo() *PluginInfo
}

// ConnectionPlugin interface for connection plugins
type ConnectionPlugin interface {
	ExecutablePlugin
	Connect(host string, options map[string]interface{}) error
	ExecuteCommand(command string) (map[string]interface{}, error)
	CopyFile(src, dest string) error
	Disconnect() error
}

// Manager interface for plugin management
type Manager interface {
	LoadModule(name string) (ExecutablePlugin, error)
	LoadPlugin(pluginType PluginType, name string) (ExecutablePlugin, error)
	GetAvailablePlugins(pluginType PluginType) ([]string, error)
	ValidatePlugin(pluginType PluginType, name string, args map[string]interface{}) error
}

// SimpleManager is a basic implementation of the Manager interface
type SimpleManager struct {
	loader *Loader
}

// NewManager creates a new plugin manager
func NewManager(loader *Loader) *SimpleManager {
	return &SimpleManager{
		loader: loader,
	}
}

// LoadModule loads a module plugin
func (m *SimpleManager) LoadModule(name string) (ExecutablePlugin, error) {
	// First try to load as a module plugin
	plugin, err := m.loader.LoadPlugin(PluginTypeModule, name)
	if err == nil && plugin.Instance != nil {
		return &PluginWrapper{plugin: plugin}, nil
	}

	// If not found, try action plugins
	plugin, err = m.loader.LoadPlugin(PluginTypeAction, name)
	if err == nil && plugin.Instance != nil {
		return &PluginWrapper{plugin: plugin}, nil
	}

	return nil, fmt.Errorf("module plugin '%s' not found", name)
}

// LoadPlugin loads a plugin by type and name
func (m *SimpleManager) LoadPlugin(pluginType PluginType, name string) (ExecutablePlugin, error) {
	plugin, err := m.loader.LoadPlugin(pluginType, name)
	if err != nil {
		return nil, err
	}

	return &PluginWrapper{plugin: plugin}, nil
}

// GetAvailablePlugins returns available plugins of a given type
func (m *SimpleManager) GetAvailablePlugins(pluginType PluginType) ([]string, error) {
	plugins, err := m.loader.ListPlugins(pluginType)
	if err != nil {
		return nil, err
	}

	names := make([]string, len(plugins))
	for i, plugin := range plugins {
		names[i] = plugin.Name
	}

	return names, nil
}

// ValidatePlugin validates a plugin with given arguments
func (m *SimpleManager) ValidatePlugin(pluginType PluginType, name string, args map[string]interface{}) error {
	plugin, err := m.LoadPlugin(pluginType, name)
	if err != nil {
		return err
	}

	return plugin.Validate(args)
}

// PluginWrapper wraps the loader's Plugin to implement our Plugin interface
type PluginWrapper struct {
	plugin *Plugin
}

func (w *PluginWrapper) Name() string {
	return w.plugin.Name
}

func (w *PluginWrapper) Type() PluginType {
	return w.plugin.Type
}

func (w *PluginWrapper) Execute(ctx context.Context, moduleCtx *ModuleContext) (map[string]interface{}, error) {
	// If the plugin instance implements our ExecutablePlugin interface, use it
	if pluginInstance, ok := w.plugin.Instance.(ExecutablePlugin); ok {
		return pluginInstance.Execute(ctx, moduleCtx)
	}

	// If the plugin instance implements ModulePlugin interface, use it
	if moduleInstance, ok := w.plugin.Instance.(ModulePlugin); ok {
		return moduleInstance.Run(ctx, moduleCtx.Args)
	}

	// For now, return a mock response
	return map[string]interface{}{
		"changed": false,
		"msg":     fmt.Sprintf("Executed plugin %s (mock implementation)", w.plugin.Name),
		"failed":  false,
	}, nil
}

func (w *PluginWrapper) GetInfo() *PluginInfo {
	return &PluginInfo{
		Name:        w.plugin.Name,
		Type:        w.plugin.Type,
		Description: w.plugin.Description,
		Version:     w.plugin.Version,
		Author:      w.plugin.Author,
	}
}

func (w *PluginWrapper) Validate(args map[string]interface{}) error {
	// If the plugin instance implements validation, use it
	if validator, ok := w.plugin.Instance.(interface{ Validate(map[string]interface{}) error }); ok {
		return validator.Validate(args)
	}

	// Default validation - just check that args is not nil
	if args == nil {
		return fmt.Errorf("plugin arguments cannot be nil")
	}

	return nil
}