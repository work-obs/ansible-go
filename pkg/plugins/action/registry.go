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

package action

import (
	"fmt"
	"sync"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// ActionPluginCreator is a function that creates an action plugin instance
type ActionPluginCreator func() plugins.ActionPlugin

// ActionPluginRegistry manages action plugin registration and creation
type ActionPluginRegistry struct {
	plugins map[string]ActionPluginCreator
	mutex   sync.RWMutex
}

// NewActionPluginRegistry creates a new action plugin registry
func NewActionPluginRegistry() *ActionPluginRegistry {
	registry := &ActionPluginRegistry{
		plugins: make(map[string]ActionPluginCreator),
	}

	// Register built-in action plugins
	registry.registerBuiltinPlugins()

	return registry
}

// Register registers an action plugin with the registry
func (r *ActionPluginRegistry) Register(name string, creator ActionPluginCreator) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.plugins[name] = creator
}

// Get retrieves an action plugin by name
func (r *ActionPluginRegistry) Get(name string) (plugins.ActionPlugin, error) {
	r.mutex.RLock()
	creator, exists := r.plugins[name]
	r.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("action plugin '%s' not found", name)
	}

	return creator(), nil
}

// List returns all registered action plugin names
func (r *ActionPluginRegistry) List() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}

// Exists checks if an action plugin is registered
func (r *ActionPluginRegistry) Exists(name string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	_, exists := r.plugins[name]
	return exists
}

// registerBuiltinPlugins registers all built-in action plugins
func (r *ActionPluginRegistry) registerBuiltinPlugins() {
	// Register setup action plugin
	r.Register("setup", func() plugins.ActionPlugin {
		return NewSetupActionPlugin()
	})

	// Register command action plugin
	r.Register("command", func() plugins.ActionPlugin {
		return NewCommandActionPlugin()
	})

	// Register shell action plugin
	r.Register("shell", func() plugins.ActionPlugin {
		return NewShellActionPlugin()
	})

	// Register copy action plugin (to be implemented)
	r.Register("copy", func() plugins.ActionPlugin {
		return NewCopyActionPlugin()
	})

	// Register file action plugin (to be implemented)
	r.Register("file", func() plugins.ActionPlugin {
		return NewFileActionPlugin()
	})

	// Register service action plugin (to be implemented)
	r.Register("service", func() plugins.ActionPlugin {
		return NewServiceActionPlugin()
	})

	// Register normal action plugin (generic action for most modules)
	r.Register("normal", func() plugins.ActionPlugin {
		return NewNormalActionPlugin()
	})
}

// Global registry instance
var DefaultRegistry = NewActionPluginRegistry()

// RegisterActionPlugin registers an action plugin with the default registry
func RegisterActionPlugin(name string, creator ActionPluginCreator) {
	DefaultRegistry.Register(name, creator)
}

// GetActionPlugin retrieves an action plugin from the default registry
func GetActionPlugin(name string) (plugins.ActionPlugin, error) {
	return DefaultRegistry.Get(name)
}

// ListActionPlugins returns all registered action plugin names
func ListActionPlugins() []string {
	return DefaultRegistry.List()
}

// ActionPluginExists checks if an action plugin exists
func ActionPluginExists(name string) bool {
	return DefaultRegistry.Exists(name)
}