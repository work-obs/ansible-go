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

package vars

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/work-obs/ansible-go/pkg/inventory"
	"github.com/work-obs/ansible-go/pkg/template"
	"gopkg.in/yaml.v3"
)

// Precedence levels for variable resolution (higher number = higher precedence)
const (
	PrecedenceInventory    = 10
	PrecedenceGroupVars    = 20
	PrecedenceHostVars     = 30
	PrecedencePlayVars     = 40
	PrecedencePlayHostVars = 50
	PrecedenceTaskVars     = 60
	PrecedenceIncludeVars  = 70
	PrecedenceSetFacts     = 80
	PrecedenceRegistered   = 90
	PrecedenceExtraVars    = 100
)

// Variable represents a variable with its value and precedence
type Variable struct {
	Name       string
	Value      interface{}
	Precedence int
	Source     string
}

// Context holds all variables for a specific execution context
type Context struct {
	Variables map[string]*Variable
	Facts     map[string]interface{}
	Hostvars  map[string]map[string]interface{}
	Groups    map[string][]string
	mutex     sync.RWMutex
}

// Manager manages variable resolution and templating
type Manager struct {
	templateEngine *template.Engine
	inventory      *inventory.Inventory
	extraVars      map[string]interface{}
	mutex          sync.RWMutex
}

// NewManager creates a new variable manager
func NewManager(inv *inventory.Inventory) *Manager {
	return &Manager{
		templateEngine: template.NewEngine(),
		inventory:      inv,
		extraVars:      make(map[string]interface{}),
	}
}

// NewContext creates a new variable context
func NewContext() *Context {
	return &Context{
		Variables: make(map[string]*Variable),
		Facts:     make(map[string]interface{}),
		Hostvars:  make(map[string]map[string]interface{}),
		Groups:    make(map[string][]string),
	}
}

// SetVariable sets a variable with the specified precedence
func (c *Context) SetVariable(name string, value interface{}, precedence int, source string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Only set if precedence is higher or variable doesn't exist
	if existing, exists := c.Variables[name]; !exists || precedence >= existing.Precedence {
		c.Variables[name] = &Variable{
			Name:       name,
			Value:      value,
			Precedence: precedence,
			Source:     source,
		}
	}
}

// GetVariable gets a variable value
func (c *Context) GetVariable(name string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if variable, exists := c.Variables[name]; exists {
		return variable.Value, true
	}
	return nil, false
}

// GetVariables returns a copy of all variables as a map
func (c *Context) GetVariables() map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	vars := make(map[string]interface{})
	for name, variable := range c.Variables {
		vars[name] = variable.Value
	}
	return vars
}

// SetFact sets a fact (highest precedence except extra vars)
func (c *Context) SetFact(name string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.Facts[name] = value
}

// GetFact gets a fact value
func (c *Context) GetFact(name string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if value, exists := c.Facts[name]; exists {
		return value, true
	}
	return nil, false
}

// SetHostvar sets a host variable
func (c *Context) SetHostvar(host, name string, value interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.Hostvars[host] == nil {
		c.Hostvars[host] = make(map[string]interface{})
	}
	c.Hostvars[host][name] = value
}

// GetHostvar gets a host variable
func (c *Context) GetHostvar(host, name string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	if hostVars, exists := c.Hostvars[host]; exists {
		if value, exists := hostVars[name]; exists {
			return value, true
		}
	}
	return nil, false
}

// SetExtraVar sets an extra variable (highest precedence)
func (m *Manager) SetExtraVar(name string, value interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.extraVars[name] = value
}

// GetExtraVars returns all extra variables
func (m *Manager) GetExtraVars() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	vars := make(map[string]interface{})
	for k, v := range m.extraVars {
		vars[k] = v
	}
	return vars
}

// CreateHostContext creates a variable context for a specific host
func (m *Manager) CreateHostContext(hostname string) (*Context, error) {
	ctx := NewContext()

	// Add inventory variables (lowest precedence)
	if m.inventory != nil {
		// Add host variables
		hostVars := m.inventory.GetHostVars(hostname)
		for name, value := range hostVars {
			ctx.SetVariable(name, value, PrecedenceHostVars, "inventory")
		}

		// Add group variables for all groups containing this host
		for groupName, group := range m.inventory.Groups {
			for _, hostName := range group.Hosts {
				if hostName == hostname {
					groupVars := m.inventory.GetGroupVars(groupName)
					for name, value := range groupVars {
						ctx.SetVariable(name, value, PrecedenceGroupVars, fmt.Sprintf("group:%s", groupName))
					}
					break
				}
			}
		}

		// Set up hostvars for all hosts
		for hostName := range m.inventory.Hosts {
			hostVars := m.inventory.GetHostVars(hostName)
			for varName, varValue := range hostVars {
				ctx.SetHostvar(hostName, varName, varValue)
			}
		}

		// Set up groups
		for groupName, group := range m.inventory.Groups {
			ctx.Groups[groupName] = append([]string{}, group.Hosts...)
		}
	}

	// Add extra variables (highest precedence)
	m.mutex.RLock()
	for name, value := range m.extraVars {
		ctx.SetVariable(name, value, PrecedenceExtraVars, "extra_vars")
	}
	m.mutex.RUnlock()

	// Add common facts
	ctx.SetFact("ansible_hostname", hostname)
	ctx.SetFact("inventory_hostname", hostname)

	return ctx, nil
}

// TemplateString renders a string template with the given context
func (m *Manager) TemplateString(templateStr string, ctx *Context) (string, error) {
	templateCtx := &template.Context{
		Variables: ctx.GetVariables(),
		Hostvars:  ctx.Hostvars,
		Groups:    ctx.Groups,
		Facts:     ctx.Facts,
	}

	// Add extra variables with highest precedence
	for name, value := range m.GetExtraVars() {
		templateCtx.Variables[name] = value
	}

	return m.templateEngine.Render(templateStr, templateCtx)
}

// TemplateValue recursively templates any string values in a complex data structure
func (m *Manager) TemplateValue(value interface{}, ctx *Context) (interface{}, error) {
	return m.templateValueRecursive(value, ctx, 0)
}

// templateValueRecursive recursively templates values with depth protection
func (m *Manager) templateValueRecursive(value interface{}, ctx *Context, depth int) (interface{}, error) {
	// Prevent infinite recursion
	if depth > 50 {
		return value, fmt.Errorf("template recursion depth exceeded")
	}

	switch v := value.(type) {
	case string:
		// Only template strings that contain template syntax
		if strings.Contains(v, "{{") || strings.Contains(v, "{%") {
			return m.TemplateString(v, ctx)
		}
		return v, nil

	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			templatedKey, err := m.templateValueRecursive(key, ctx, depth+1)
			if err != nil {
				return nil, fmt.Errorf("failed to template key '%s': %w", key, err)
			}

			templatedVal, err := m.templateValueRecursive(val, ctx, depth+1)
			if err != nil {
				return nil, fmt.Errorf("failed to template value for key '%s': %w", key, err)
			}

			keyStr, ok := templatedKey.(string)
			if !ok {
				keyStr = fmt.Sprintf("%v", templatedKey)
			}
			result[keyStr] = templatedVal
		}
		return result, nil

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			templatedItem, err := m.templateValueRecursive(item, ctx, depth+1)
			if err != nil {
				return nil, fmt.Errorf("failed to template array item %d: %w", i, err)
			}
			result[i] = templatedItem
		}
		return result, nil

	default:
		// Return non-string values as-is
		return value, nil
	}
}

// MergeContext merges variables from another context
func (c *Context) MergeContext(other *Context) {
	c.mutex.Lock()
	other.mutex.RLock()
	defer c.mutex.Unlock()
	defer other.mutex.RUnlock()

	// Merge variables respecting precedence
	for name, variable := range other.Variables {
		if existing, exists := c.Variables[name]; !exists || variable.Precedence >= existing.Precedence {
			c.Variables[name] = &Variable{
				Name:       variable.Name,
				Value:      variable.Value,
				Precedence: variable.Precedence,
				Source:     variable.Source,
			}
		}
	}

	// Merge facts
	for name, value := range other.Facts {
		c.Facts[name] = value
	}

	// Merge hostvars
	for host, hostVars := range other.Hostvars {
		if c.Hostvars[host] == nil {
			c.Hostvars[host] = make(map[string]interface{})
		}
		for name, value := range hostVars {
			c.Hostvars[host][name] = value
		}
	}

	// Merge groups
	for group, hosts := range other.Groups {
		c.Groups[group] = append([]string{}, hosts...)
	}
}

// ConvertToTypedValue attempts to convert a value to a more specific type
func ConvertToTypedValue(value interface{}) interface{} {
	strValue, ok := value.(string)
	if !ok {
		return value
	}

	// Try to convert to boolean
	switch strings.ToLower(strValue) {
	case "true", "yes", "on":
		return true
	case "false", "no", "off":
		return false
	}

	// Try to convert to integer
	if intValue, err := strconv.ParseInt(strValue, 10, 64); err == nil {
		// Return as int if it fits in int range, otherwise int64
		if intValue >= int64(int(^uint(0)>>1)*-1) && intValue <= int64(int(^uint(0)>>1)) {
			return int(intValue)
		}
		return intValue
	}

	// Try to convert to float
	if floatValue, err := strconv.ParseFloat(strValue, 64); err == nil {
		return floatValue
	}

	// Return as string if no conversion is possible
	return value
}

// LoadVarsFromYAML loads variables from YAML data
func LoadVarsFromYAML(data []byte) (map[string]interface{}, error) {
	var vars map[string]interface{}
	if err := yaml.Unmarshal(data, &vars); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert string values to appropriate types
	convertedVars := make(map[string]interface{})
	for key, value := range vars {
		convertedVars[key] = ConvertToTypedValue(value)
	}

	return convertedVars, nil
}

// DotNotationGet retrieves a value using dot notation (e.g., "user.name")
func (c *Context) DotNotationGet(path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, false
	}

	// Start with the first part as a variable
	current, exists := c.GetVariable(parts[0])
	if !exists {
		return nil, false
	}

	// Navigate through the path
	for _, part := range parts[1:] {
		switch v := current.(type) {
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil, false
			}
		case map[interface{}]interface{}:
			if val, ok := v[part]; ok {
				current = val
			} else {
				return nil, false
			}
		default:
			// Try to use reflection to access struct fields
			if reflectValue := reflect.ValueOf(current); reflectValue.IsValid() {
				if reflectValue.Kind() == reflect.Ptr {
					reflectValue = reflectValue.Elem()
				}
				if reflectValue.Kind() == reflect.Struct {
					fieldValue := reflectValue.FieldByName(part)
					if fieldValue.IsValid() {
						current = fieldValue.Interface()
					} else {
						return nil, false
					}
				} else {
					return nil, false
				}
			} else {
				return nil, false
			}
		}
	}

	return current, true
}

// DotNotationSet sets a value using dot notation (e.g., "user.name")
func (c *Context) DotNotationSet(path string, value interface{}, precedence int, source string) error {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return fmt.Errorf("empty path")
	}

	if len(parts) == 1 {
		// Simple variable
		c.SetVariable(path, value, precedence, source)
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Navigate to the parent and create intermediate maps if necessary
	rootName := parts[0]
	var root interface{}

	if variable, exists := c.Variables[rootName]; exists {
		root = variable.Value
	} else {
		root = make(map[string]interface{})
	}

	current := root
	for i, part := range parts[1 : len(parts)-1] {
		switch v := current.(type) {
		case map[string]interface{}:
			if _, exists := v[part]; !exists {
				v[part] = make(map[string]interface{})
			}
			current = v[part]
		default:
			return fmt.Errorf("cannot navigate path %s: element at %s is not a map", path, strings.Join(parts[:i+2], "."))
		}
	}

	// Set the final value
	finalPart := parts[len(parts)-1]
	if currentMap, ok := current.(map[string]interface{}); ok {
		currentMap[finalPart] = value

		// Update the root variable
		c.Variables[rootName] = &Variable{
			Name:       rootName,
			Value:      root,
			Precedence: precedence,
			Source:     source,
		}
	} else {
		return fmt.Errorf("cannot set value at path %s: parent is not a map", path)
	}

	return nil
}

// GetVariablesWithSource returns all variables with their source information
func (c *Context) GetVariablesWithSource() map[string]*Variable {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	vars := make(map[string]*Variable)
	for name, variable := range c.Variables {
		vars[name] = &Variable{
			Name:       variable.Name,
			Value:      variable.Value,
			Precedence: variable.Precedence,
			Source:     variable.Source,
		}
	}
	return vars
}

// Clone creates a deep copy of the context
func (c *Context) Clone() *Context {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	clone := NewContext()

	// Clone variables
	for name, variable := range c.Variables {
		clone.Variables[name] = &Variable{
			Name:       variable.Name,
			Value:      deepCopyValue(variable.Value),
			Precedence: variable.Precedence,
			Source:     variable.Source,
		}
	}

	// Clone facts
	for name, value := range c.Facts {
		clone.Facts[name] = deepCopyValue(value)
	}

	// Clone hostvars
	for host, hostVars := range c.Hostvars {
		clone.Hostvars[host] = make(map[string]interface{})
		for name, value := range hostVars {
			clone.Hostvars[host][name] = deepCopyValue(value)
		}
	}

	// Clone groups
	for group, hosts := range c.Groups {
		clone.Groups[group] = append([]string{}, hosts...)
	}

	return clone
}

// deepCopyValue creates a deep copy of a value
func deepCopyValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		copy := make(map[string]interface{})
		for key, val := range v {
			copy[key] = deepCopyValue(val)
		}
		return copy
	case []interface{}:
		copy := make([]interface{}, len(v))
		for i, item := range v {
			copy[i] = deepCopyValue(item)
		}
		return copy
	case map[interface{}]interface{}:
		copy := make(map[interface{}]interface{})
		for key, val := range v {
			copy[key] = deepCopyValue(val)
		}
		return copy
	default:
		// For primitive types, return as-is (they're immutable in Go)
		return value
	}
}

// FilterVariables returns variables matching a pattern
func (c *Context) FilterVariables(pattern string) map[string]interface{} {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	filtered := make(map[string]interface{})

	for name, variable := range c.Variables {
		if matchesPattern(name, pattern) {
			filtered[name] = variable.Value
		}
	}

	return filtered
}

// matchesPattern checks if a string matches a simple glob pattern
func matchesPattern(text, pattern string) bool {
	// Simple pattern matching with * wildcard
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return text == pattern
	}

	// Convert glob pattern to regex-like matching
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		prefix, suffix := parts[0], parts[1]
		return strings.HasPrefix(text, prefix) && strings.HasSuffix(text, suffix)
	}

	// For more complex patterns, use basic matching
	return strings.Contains(text, strings.ReplaceAll(pattern, "*", ""))
}