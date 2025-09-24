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

package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// CachePlugin interface for cache plugins
type CachePlugin interface {
	plugins.BasePlugin
	Get(key string) (interface{}, error)
	Set(key string, value interface{}, ttl time.Duration) error
	Delete(key string) error
	Contains(key string) bool
	Keys() ([]string, error)
	Flush() error
}

// BaseCachePlugin provides common functionality for cache plugins
type BaseCachePlugin struct {
	name        string
	description string
	version     string
	author      string
}

func NewBaseCachePlugin(name, description, version, author string) *BaseCachePlugin {
	return &BaseCachePlugin{
		name:        name,
		description: description,
		version:     version,
		author:      author,
	}
}

func (c *BaseCachePlugin) Name() string {
	return c.name
}

func (c *BaseCachePlugin) Type() plugins.PluginType {
	return plugins.PluginTypeCache
}

func (c *BaseCachePlugin) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        c.name,
		Type:        plugins.PluginTypeCache,
		Description: c.description,
		Version:     c.version,
		Author:      []string{c.author},
	}
}

// MemoryCachePlugin implements in-memory caching
type MemoryCachePlugin struct {
	*BaseCachePlugin
	cache map[string]*cacheEntry
	mutex sync.RWMutex
}

type cacheEntry struct {
	value  interface{}
	expiry time.Time
}

func NewMemoryCachePlugin() *MemoryCachePlugin {
	return &MemoryCachePlugin{
		BaseCachePlugin: NewBaseCachePlugin(
			"memory",
			"In-memory cache plugin",
			"1.0.0",
			"Ansible Project",
		),
		cache: make(map[string]*cacheEntry),
	}
}

func (m *MemoryCachePlugin) Get(key string) (interface{}, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	entry, exists := m.cache[key]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", key)
	}

	if time.Now().After(entry.expiry) {
		delete(m.cache, key)
		return nil, fmt.Errorf("key expired: %s", key)
	}

	return entry.value, nil
}

func (m *MemoryCachePlugin) Set(key string, value interface{}, ttl time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	expiry := time.Now().Add(ttl)
	if ttl == 0 {
		expiry = time.Time{} // Never expires
	}

	m.cache[key] = &cacheEntry{
		value:  value,
		expiry: expiry,
	}

	return nil
}

func (m *MemoryCachePlugin) Delete(key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.cache, key)
	return nil
}

func (m *MemoryCachePlugin) Contains(key string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	entry, exists := m.cache[key]
	if !exists {
		return false
	}

	if time.Now().After(entry.expiry) {
		delete(m.cache, key)
		return false
	}

	return true
}

func (m *MemoryCachePlugin) Keys() ([]string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	keys := make([]string, 0, len(m.cache))
	now := time.Now()

	for key, entry := range m.cache {
		if now.After(entry.expiry) {
			continue
		}
		keys = append(keys, key)
	}

	return keys, nil
}

func (m *MemoryCachePlugin) Flush() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.cache = make(map[string]*cacheEntry)
	return nil
}

// JsonFileCachePlugin implements file-based JSON caching
type JsonFileCachePlugin struct {
	*BaseCachePlugin
	cacheDir string
	mutex    sync.RWMutex
}

func NewJsonFileCachePlugin(cacheDir string) *JsonFileCachePlugin {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "ansible-cache")
	}

	return &JsonFileCachePlugin{
		BaseCachePlugin: NewBaseCachePlugin(
			"jsonfile",
			"JSON file cache plugin",
			"1.0.0",
			"Ansible Project",
		),
		cacheDir: cacheDir,
	}
}

func (j *JsonFileCachePlugin) getCacheFile(key string) string {
	return filepath.Join(j.cacheDir, key+".json")
}

func (j *JsonFileCachePlugin) Get(key string) (interface{}, error) {
	j.mutex.RLock()
	defer j.mutex.RUnlock()

	filename := j.getCacheFile(key)
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("key not found: %s", key)
		}
		return nil, err
	}

	var cacheData struct {
		Value  interface{} `json:"value"`
		Expiry time.Time   `json:"expiry"`
	}

	if err := json.Unmarshal(data, &cacheData); err != nil {
		return nil, err
	}

	if !cacheData.Expiry.IsZero() && time.Now().After(cacheData.Expiry) {
		os.Remove(filename)
		return nil, fmt.Errorf("key expired: %s", key)
	}

	return cacheData.Value, nil
}

func (j *JsonFileCachePlugin) Set(key string, value interface{}, ttl time.Duration) error {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	// Ensure cache directory exists
	if err := os.MkdirAll(j.cacheDir, 0755); err != nil {
		return err
	}

	expiry := time.Time{}
	if ttl > 0 {
		expiry = time.Now().Add(ttl)
	}

	cacheData := struct {
		Value  interface{} `json:"value"`
		Expiry time.Time   `json:"expiry"`
	}{
		Value:  value,
		Expiry: expiry,
	}

	data, err := json.Marshal(cacheData)
	if err != nil {
		return err
	}

	filename := j.getCacheFile(key)
	return os.WriteFile(filename, data, 0644)
}

func (j *JsonFileCachePlugin) Delete(key string) error {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	filename := j.getCacheFile(key)
	err := os.Remove(filename)
	if os.IsNotExist(err) {
		return nil // Key doesn't exist, which is fine for delete
	}
	return err
}

func (j *JsonFileCachePlugin) Contains(key string) bool {
	filename := j.getCacheFile(key)
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return false
	}

	// Check if expired
	_, err := j.Get(key)
	return err == nil
}

func (j *JsonFileCachePlugin) Keys() ([]string, error) {
	j.mutex.RLock()
	defer j.mutex.RUnlock()

	entries, err := os.ReadDir(j.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if filepath.Ext(name) == ".json" {
			key := name[:len(name)-5] // Remove .json extension
			if j.Contains(key) {
				keys = append(keys, key)
			}
		}
	}

	return keys, nil
}

func (j *JsonFileCachePlugin) Flush() error {
	j.mutex.Lock()
	defer j.mutex.Unlock()

	entries, err := os.ReadDir(j.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			os.Remove(filepath.Join(j.cacheDir, entry.Name()))
		}
	}

	return nil
}

// CachePluginRegistry manages cache plugin registration and creation
type CachePluginRegistry struct {
	plugins map[string]func() CachePlugin
}

func NewCachePluginRegistry() *CachePluginRegistry {
	registry := &CachePluginRegistry{
		plugins: make(map[string]func() CachePlugin),
	}

	// Register built-in cache plugins
	registry.Register("memory", func() CachePlugin { return NewMemoryCachePlugin() })
	registry.Register("jsonfile", func() CachePlugin { return NewJsonFileCachePlugin("") })

	return registry
}

func (r *CachePluginRegistry) Register(name string, creator func() CachePlugin) {
	r.plugins[name] = creator
}

func (r *CachePluginRegistry) Get(name string) (CachePlugin, error) {
	creator, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("cache plugin '%s' not found", name)
	}
	return creator(), nil
}

func (r *CachePluginRegistry) Exists(name string) bool {
	_, exists := r.plugins[name]
	return exists
}

func (r *CachePluginRegistry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}