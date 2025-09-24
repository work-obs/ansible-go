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
	"fmt"
	"path/filepath"
	"plugin"
	"reflect"
	"strings"
	"sync"

	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/spf13/afero"
)


// Plugin represents a loaded plugin
type Plugin struct {
	Name        string
	Type        PluginType
	Path        string
	Description string
	Version     string
	Author      []string
	Instance    interface{}
}

// PluginInterface defines the base interface that all plugins must implement
type PluginInterface interface {
	Name() string
	Description() string
	Version() string
	Author() []string
}

// Loader manages plugin loading and caching
type Loader struct {
	config      *config.Config
	fs          afero.Fs
	pluginCache map[string]*Plugin
	pathCache   map[PluginType][]string
	mutex       sync.RWMutex
}

// NewLoader creates a new plugin loader
func NewLoader(config *config.Config, fs afero.Fs) *Loader {
	return &Loader{
		config:      config,
		fs:          fs,
		pluginCache: make(map[string]*Plugin),
		pathCache:   make(map[PluginType][]string),
	}
}

// LoadPlugin loads a plugin by name and type
func (l *Loader) LoadPlugin(pluginType PluginType, name string) (*Plugin, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	cacheKey := fmt.Sprintf("%s:%s", pluginType, name)

	// Check cache first
	if plugin, exists := l.pluginCache[cacheKey]; exists {
		return plugin, nil
	}

	// Find plugin path
	pluginPath, err := l.findPlugin(pluginType, name)
	if err != nil {
		return nil, fmt.Errorf("plugin not found: %w", err)
	}

	// Load the plugin
	loadedPlugin, err := l.loadPluginFromPath(pluginType, name, pluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load plugin: %w", err)
	}

	// Cache the plugin
	l.pluginCache[cacheKey] = loadedPlugin

	return loadedPlugin, nil
}

// ListPlugins returns all available plugins of a given type
func (l *Loader) ListPlugins(pluginType PluginType) ([]*Plugin, error) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	var plugins []*Plugin

	// Get search paths for the plugin type
	searchPaths := l.getPluginPaths(pluginType)

	for _, path := range searchPaths {
		pluginNames, err := l.discoverPluginsInPath(path, pluginType)
		if err != nil {
			continue // Skip paths with errors
		}

		for _, name := range pluginNames {
			plugin, err := l.LoadPlugin(pluginType, name)
			if err != nil {
				continue // Skip plugins that fail to load
			}
			plugins = append(plugins, plugin)
		}
	}

	return plugins, nil
}

// findPlugin finds a plugin file by name and type
func (l *Loader) findPlugin(pluginType PluginType, name string) (string, error) {
	searchPaths := l.getPluginPaths(pluginType)

	for _, path := range searchPaths {
		// Try different file extensions
		extensions := []string{".so", ".py", ".go"}
		for _, ext := range extensions {
			pluginPath := filepath.Join(path, name+ext)
			if exists, _ := afero.Exists(l.fs, pluginPath); exists {
				return pluginPath, nil
			}
		}

		// Also try directory-based plugins
		dirPath := filepath.Join(path, name)
		if exists, _ := afero.DirExists(l.fs, dirPath); exists {
			return dirPath, nil
		}
	}

	return "", fmt.Errorf("plugin '%s' of type '%s' not found", name, pluginType)
}

// loadPluginFromPath loads a plugin from a specific path
func (l *Loader) loadPluginFromPath(pluginType PluginType, name, path string) (*Plugin, error) {
	ext := filepath.Ext(path)

	switch ext {
	case ".so":
		return l.loadGoPlugin(pluginType, name, path)
	case ".py":
		return l.loadPythonPlugin(pluginType, name, path)
	default:
		// Try to determine plugin type from directory structure
		return l.loadDirectoryPlugin(pluginType, name, path)
	}
}

// loadGoPlugin loads a Go-based plugin (.so file)
func (l *Loader) loadGoPlugin(pluginType PluginType, name, path string) (*Plugin, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open plugin: %w", err)
	}

	// Look for the plugin entry point
	symbol, err := p.Lookup("New" + strings.Title(string(pluginType)) + "Plugin")
	if err != nil {
		return nil, fmt.Errorf("plugin entry point not found: %w", err)
	}

	// Create plugin instance
	pluginFactory, ok := symbol.(func() PluginInterface)
	if !ok {
		return nil, fmt.Errorf("invalid plugin factory function")
	}

	instance := pluginFactory()

	return &Plugin{
		Name:        name,
		Type:        pluginType,
		Path:        path,
		Description: instance.Description(),
		Version:     instance.Version(),
		Author:      instance.Author(),
		Instance:    instance,
	}, nil
}

// loadPythonPlugin loads a Python-based plugin
func (l *Loader) loadPythonPlugin(pluginType PluginType, name, path string) (*Plugin, error) {
	// This would integrate with a Python interpreter
	// For now, return a placeholder
	return &Plugin{
		Name:        name,
		Type:        pluginType,
		Path:        path,
		Description: "Python plugin (not yet implemented)",
		Version:     "unknown",
		Author:      []string{"unknown"},
		Instance:    nil,
	}, fmt.Errorf("Python plugin loading not yet implemented")
}

// loadDirectoryPlugin loads a directory-based plugin
func (l *Loader) loadDirectoryPlugin(pluginType PluginType, name, path string) (*Plugin, error) {
	// This would handle complex plugin structures
	return &Plugin{
		Name:        name,
		Type:        pluginType,
		Path:        path,
		Description: "Directory plugin",
		Version:     "unknown",
		Author:      []string{"unknown"},
		Instance:    nil,
	}, nil
}

// getPluginPaths returns search paths for a specific plugin type
func (l *Loader) getPluginPaths(pluginType PluginType) []string {
	if paths, exists := l.pathCache[pluginType]; exists {
		return paths
	}

	var paths []string

	// Add built-in plugin paths
	paths = append(paths, filepath.Join("plugins", string(pluginType)))

	// Add configuration-specified paths
	switch pluginType {
	case PluginTypeAction:
		paths = append(paths, l.config.ActionPluginPath...)
	case PluginTypeCallback:
		paths = append(paths, l.config.CallbackPluginPath...)
	case PluginTypeConnection:
		paths = append(paths, l.config.ConnectionPluginPath...)
	case PluginTypeLookup:
		paths = append(paths, l.config.LookupPluginPath...)
	case PluginTypeFilter:
		paths = append(paths, l.config.FilterPluginPath...)
	case PluginTypeTest:
		paths = append(paths, l.config.TestPluginPath...)
	case PluginTypeStrategy:
		paths = append(paths, l.config.StrategyPluginPath...)
	case PluginTypeVars:
		paths = append(paths, l.config.VarsPluginPath...)
	case PluginTypeCache:
		paths = append(paths, l.config.CachePluginPath...)
	}

	// Cache the paths
	l.pathCache[pluginType] = paths

	return paths
}

// discoverPluginsInPath discovers all plugins in a given path
func (l *Loader) discoverPluginsInPath(path string, pluginType PluginType) ([]string, error) {
	var pluginNames []string

	entries, err := afero.ReadDir(l.fs, path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		name := entry.Name()

		if entry.IsDir() {
			// Directory-based plugin
			pluginNames = append(pluginNames, name)
		} else {
			// File-based plugin
			ext := filepath.Ext(name)
			if ext == ".so" || ext == ".py" || ext == ".go" {
				pluginName := strings.TrimSuffix(name, ext)
				pluginNames = append(pluginNames, pluginName)
			}
		}
	}

	return pluginNames, nil
}

// GetPlugin returns a cached plugin instance
func (l *Loader) GetPlugin(pluginType PluginType, name string) (*Plugin, bool) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	cacheKey := fmt.Sprintf("%s:%s", pluginType, name)
	plugin, exists := l.pluginCache[cacheKey]
	return plugin, exists
}

// ClearCache clears the plugin cache
func (l *Loader) ClearCache() {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.pluginCache = make(map[string]*Plugin)
	l.pathCache = make(map[PluginType][]string)
}

// GetPluginInstance returns a typed plugin instance
func GetPluginInstance[T any](plugin *Plugin) (T, error) {
	var zero T

	if plugin.Instance == nil {
		return zero, fmt.Errorf("plugin instance is nil")
	}

	instance, ok := plugin.Instance.(T)
	if !ok {
		return zero, fmt.Errorf("plugin instance is not of expected type %T", zero)
	}

	return instance, nil
}

// ValidatePlugin validates that a plugin implements the required interface
func ValidatePlugin(plugin *Plugin, expectedInterface reflect.Type) error {
	if plugin.Instance == nil {
		return fmt.Errorf("plugin instance is nil")
	}

	instanceType := reflect.TypeOf(plugin.Instance)
	if !instanceType.Implements(expectedInterface) {
		return fmt.Errorf("plugin does not implement required interface")
	}

	return nil
}