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

package router

import (
	"fmt"
	"sync"

	"github.com/work-obs/ansible-go/pkg/plugins"
	"gopkg.in/yaml.v3"
)

// ModuleRouting represents module routing configuration
type ModuleRouting struct {
	Redirect     string `yaml:"redirect,omitempty"`
	Deprecation  string `yaml:"deprecation,omitempty"`
	Removal      string `yaml:"removal,omitempty"`
	WarningText  string `yaml:"warning_text,omitempty"`
	ActionPlugin string `yaml:"action_plugin,omitempty"`
}

// PluginRouting represents the complete plugin routing configuration
type PluginRouting struct {
	Modules    map[string]ModuleRouting `yaml:"modules,omitempty"`
	Connection map[string]ModuleRouting `yaml:"connection,omitempty"`
	Action     map[string]ModuleRouting `yaml:"action,omitempty"`
	Become     map[string]ModuleRouting `yaml:"become,omitempty"`
	Cache      map[string]ModuleRouting `yaml:"cache,omitempty"`
	Callback   map[string]ModuleRouting `yaml:"callback,omitempty"`
	Cliconf    map[string]ModuleRouting `yaml:"cliconf,omitempty"`
	Filter     map[string]ModuleRouting `yaml:"filter,omitempty"`
	Httpapi    map[string]ModuleRouting `yaml:"httpapi,omitempty"`
	Inventory  map[string]ModuleRouting `yaml:"inventory,omitempty"`
	Lookup     map[string]ModuleRouting `yaml:"lookup,omitempty"`
	Netconf    map[string]ModuleRouting `yaml:"netconf,omitempty"`
	Shell      map[string]ModuleRouting `yaml:"shell,omitempty"`
	Strategy   map[string]ModuleRouting `yaml:"strategy,omitempty"`
	Terminal   map[string]ModuleRouting `yaml:"terminal,omitempty"`
	Test       map[string]ModuleRouting `yaml:"test,omitempty"`
	Vars       map[string]ModuleRouting `yaml:"vars,omitempty"`
}

// RuntimeConfig represents the complete runtime configuration
type RuntimeConfig struct {
	PluginRouting   PluginRouting `yaml:"plugin_routing"`
	RequiresPython  string        `yaml:"requires_ansible,omitempty"`
	ActionGroups    map[string][]string `yaml:"action_groups,omitempty"`
}

// Router manages programmable routing for modules and plugins
type Router struct {
	config      *RuntimeConfig
	pluginCache map[string]string
	mutex       sync.RWMutex
}

// NewRouter creates a new programmable router
func NewRouter() *Router {
	return &Router{
		config:      &RuntimeConfig{},
		pluginCache: make(map[string]string),
	}
}

// LoadConfig loads routing configuration from YAML data
func (r *Router) LoadConfig(data []byte) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	config := &RuntimeConfig{}
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to unmarshal routing config: %w", err)
	}

	r.config = config
	r.pluginCache = make(map[string]string) // Clear cache

	return nil
}

// LoadConfigFromMap loads routing configuration from a map
func (r *Router) LoadConfigFromMap(configMap map[string]interface{}) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Convert map to YAML and back to ensure proper typing
	yamlData, err := yaml.Marshal(configMap)
	if err != nil {
		return fmt.Errorf("failed to marshal config map: %w", err)
	}

	config := &RuntimeConfig{}
	if err := yaml.Unmarshal(yamlData, config); err != nil {
		return fmt.Errorf("failed to unmarshal config map: %w", err)
	}

	r.config = config
	r.pluginCache = make(map[string]string) // Clear cache

	return nil
}

// GetConfig returns the current configuration
func (r *Router) GetConfig() *RuntimeConfig {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.config
}

// GetConfigAsMap returns the current configuration as a map
func (r *Router) GetConfigAsMap() (map[string]interface{}, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Convert to YAML first, then to map to ensure proper structure
	yamlData, err := yaml.Marshal(r.config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}

	var configMap map[string]interface{}
	if err := yaml.Unmarshal(yamlData, &configMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to map: %w", err)
	}

	return configMap, nil
}

// ResolveModule resolves a module name to its actual implementation
func (r *Router) ResolveModule(moduleName string) (string, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Check cache first
	if resolved, exists := r.pluginCache[moduleName]; exists {
		return resolved, nil
	}

	// Check for module routing
	if routing, exists := r.config.PluginRouting.Modules[moduleName]; exists {
		if routing.Redirect != "" {
			r.pluginCache[moduleName] = routing.Redirect
			return routing.Redirect, nil
		}
	}

	// No routing found, return original name
	r.pluginCache[moduleName] = moduleName
	return moduleName, nil
}

// ResolvePlugin resolves a plugin name to its actual implementation
func (r *Router) ResolvePlugin(pluginType plugins.PluginType, pluginName string) (string, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	cacheKey := fmt.Sprintf("%s:%s", pluginType, pluginName)

	// Check cache first
	if resolved, exists := r.pluginCache[cacheKey]; exists {
		return resolved, nil
	}

	var routing *ModuleRouting
	var exists bool

	// Get routing based on plugin type
	switch pluginType {
	case plugins.PluginTypeConnection:
		if r, ok := r.config.PluginRouting.Connection[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeAction:
		if r, ok := r.config.PluginRouting.Action[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeBecome:
		if r, ok := r.config.PluginRouting.Become[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeCache:
		if r, ok := r.config.PluginRouting.Cache[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeCallback:
		if r, ok := r.config.PluginRouting.Callback[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeFilter:
		if r, ok := r.config.PluginRouting.Filter[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeInventory:
		if r, ok := r.config.PluginRouting.Inventory[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeLookup:
		if r, ok := r.config.PluginRouting.Lookup[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeShell:
		if r, ok := r.config.PluginRouting.Shell[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeStrategy:
		if r, ok := r.config.PluginRouting.Strategy[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeTerminal:
		if r, ok := r.config.PluginRouting.Terminal[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeTest:
		if r, ok := r.config.PluginRouting.Test[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeVars:
		if r, ok := r.config.PluginRouting.Vars[pluginName]; ok {
			routing = &r
			exists = true
		}
	default:
		exists = false
	}

	resolved := pluginName
	if exists && routing != nil && routing.Redirect != "" {
		resolved = routing.Redirect
	}

	// Cache the result
	r.pluginCache[cacheKey] = resolved
	return resolved, nil
}

// GetModuleActionPlugin returns the action plugin for a module
func (r *Router) GetModuleActionPlugin(moduleName string) (string, bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if routing, exists := r.config.PluginRouting.Modules[moduleName]; exists {
		if routing.ActionPlugin != "" {
			return routing.ActionPlugin, true
		}
		// If there's a redirect, check if that module has an action plugin
		if routing.Redirect != "" {
			if redirectRouting, redirectExists := r.config.PluginRouting.Modules[routing.Redirect]; redirectExists {
				if redirectRouting.ActionPlugin != "" {
					return redirectRouting.ActionPlugin, true
				}
			}
		}
	}

	return "", false
}

// IsModuleDeprecated checks if a module is deprecated
func (r *Router) IsModuleDeprecated(moduleName string) (bool, string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if routing, exists := r.config.PluginRouting.Modules[moduleName]; exists {
		if routing.Deprecation != "" {
			warningText := routing.WarningText
			if warningText == "" {
				warningText = fmt.Sprintf("Module '%s' is deprecated", moduleName)
			}
			return true, warningText
		}
	}

	return false, ""
}

// IsPluginDeprecated checks if a plugin is deprecated
func (r *Router) IsPluginDeprecated(pluginType plugins.PluginType, pluginName string) (bool, string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var routing *ModuleRouting
	var exists bool

	switch pluginType {
	case plugins.PluginTypeConnection:
		if r, ok := r.config.PluginRouting.Connection[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeAction:
		if r, ok := r.config.PluginRouting.Action[pluginName]; ok {
			routing = &r
			exists = true
		}
	case plugins.PluginTypeCallback:
		if r, ok := r.config.PluginRouting.Callback[pluginName]; ok {
			routing = &r
			exists = true
		}
	// Add other types as needed
	default:
		exists = false
	}

	if exists && routing != nil && routing.Deprecation != "" {
		warningText := routing.WarningText
		if warningText == "" {
			warningText = fmt.Sprintf("Plugin '%s' of type '%s' is deprecated", pluginName, pluginType)
		}
		return true, warningText
	}

	return false, ""
}

// GetActionGroups returns action groups configuration
func (r *Router) GetActionGroups() map[string][]string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.config.ActionGroups
}

// IsModuleInActionGroup checks if a module belongs to an action group
func (r *Router) IsModuleInActionGroup(moduleName, groupName string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if group, exists := r.config.ActionGroups[groupName]; exists {
		for _, module := range group {
			if module == moduleName {
				return true
			}
		}
	}

	return false
}

// ClearCache clears the internal routing cache
func (r *Router) ClearCache() {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.pluginCache = make(map[string]string)
}

// ValidateConfig validates the routing configuration
func (r *Router) ValidateConfig() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Basic validation - check for circular redirects
	visited := make(map[string]bool)

	for moduleName := range r.config.PluginRouting.Modules {
		if err := r.checkCircularRedirect(moduleName, visited, make(map[string]bool)); err != nil {
			return err
		}
	}

	return nil
}

// checkCircularRedirect checks for circular redirects in module routing
func (r *Router) checkCircularRedirect(moduleName string, visited, currentPath map[string]bool) error {
	if currentPath[moduleName] {
		return fmt.Errorf("circular redirect detected for module '%s'", moduleName)
	}

	if visited[moduleName] {
		return nil
	}

	routing, exists := r.config.PluginRouting.Modules[moduleName]
	if !exists || routing.Redirect == "" {
		visited[moduleName] = true
		return nil
	}

	currentPath[moduleName] = true
	err := r.checkCircularRedirect(routing.Redirect, visited, currentPath)
	delete(currentPath, moduleName)

	if err != nil {
		return err
	}

	visited[moduleName] = true
	return nil
}