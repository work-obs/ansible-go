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
	"testing"

	"github.com/work-obs/ansible-go/pkg/inventory"
	"github.com/spf13/afero"
)

func TestNewContext(t *testing.T) {
	ctx := NewContext()

	if ctx == nil {
		t.Fatal("Expected non-nil context")
	}

	if ctx.Variables == nil {
		t.Error("Expected variables map to be initialized")
	}

	if ctx.Facts == nil {
		t.Error("Expected facts map to be initialized")
	}

	if ctx.Hostvars == nil {
		t.Error("Expected hostvars map to be initialized")
	}

	if ctx.Groups == nil {
		t.Error("Expected groups map to be initialized")
	}
}

func TestContext_SetVariable(t *testing.T) {
	ctx := NewContext()

	// Test setting a new variable
	ctx.SetVariable("test_var", "test_value", PrecedenceTaskVars, "test")

	value, exists := ctx.GetVariable("test_var")
	if !exists {
		t.Fatal("Expected variable to exist")
	}

	if value != "test_value" {
		t.Errorf("Expected 'test_value', got '%v'", value)
	}

	// Test precedence - higher precedence should override
	ctx.SetVariable("test_var", "high_precedence", PrecedenceExtraVars, "extra_vars")

	value, exists = ctx.GetVariable("test_var")
	if !exists {
		t.Fatal("Expected variable to exist")
	}

	if value != "high_precedence" {
		t.Errorf("Expected 'high_precedence', got '%v'", value)
	}

	// Test precedence - lower precedence should not override
	ctx.SetVariable("test_var", "low_precedence", PrecedenceGroupVars, "group_vars")

	value, exists = ctx.GetVariable("test_var")
	if !exists {
		t.Fatal("Expected variable to exist")
	}

	if value != "high_precedence" {
		t.Errorf("Expected 'high_precedence' to remain, got '%v'", value)
	}
}

func TestContext_SetFact(t *testing.T) {
	ctx := NewContext()

	ctx.SetFact("test_fact", "fact_value")

	value, exists := ctx.GetFact("test_fact")
	if !exists {
		t.Fatal("Expected fact to exist")
	}

	if value != "fact_value" {
		t.Errorf("Expected 'fact_value', got '%v'", value)
	}

	// Test non-existent fact
	_, exists = ctx.GetFact("nonexistent")
	if exists {
		t.Error("Expected nonexistent fact to not exist")
	}
}

func TestContext_SetHostvar(t *testing.T) {
	ctx := NewContext()

	ctx.SetHostvar("host1", "var1", "value1")
	ctx.SetHostvar("host1", "var2", "value2")
	ctx.SetHostvar("host2", "var1", "value3")

	// Test getting hostvar
	value, exists := ctx.GetHostvar("host1", "var1")
	if !exists {
		t.Fatal("Expected hostvar to exist")
	}

	if value != "value1" {
		t.Errorf("Expected 'value1', got '%v'", value)
	}

	// Test getting hostvar for different host
	value, exists = ctx.GetHostvar("host2", "var1")
	if !exists {
		t.Fatal("Expected hostvar to exist")
	}

	if value != "value3" {
		t.Errorf("Expected 'value3', got '%v'", value)
	}

	// Test non-existent hostvar
	_, exists = ctx.GetHostvar("host1", "nonexistent")
	if exists {
		t.Error("Expected nonexistent hostvar to not exist")
	}

	_, exists = ctx.GetHostvar("nonexistent_host", "var1")
	if exists {
		t.Error("Expected hostvar for nonexistent host to not exist")
	}
}

func TestNewManager(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := inventory.NewInventory(fs)
	manager := NewManager(inv)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.templateEngine == nil {
		t.Error("Expected template engine to be initialized")
	}

	if manager.inventory != inv {
		t.Error("Expected inventory to be set correctly")
	}

	if manager.extraVars == nil {
		t.Error("Expected extra vars to be initialized")
	}
}

func TestManager_SetExtraVar(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := inventory.NewInventory(fs)
	manager := NewManager(inv)

	manager.SetExtraVar("extra1", "value1")
	manager.SetExtraVar("extra2", 42)

	extraVars := manager.GetExtraVars()

	if len(extraVars) != 2 {
		t.Errorf("Expected 2 extra vars, got %d", len(extraVars))
	}

	if extraVars["extra1"] != "value1" {
		t.Errorf("Expected 'value1', got '%v'", extraVars["extra1"])
	}

	if extraVars["extra2"] != 42 {
		t.Errorf("Expected 42, got '%v'", extraVars["extra2"])
	}
}

func TestManager_CreateHostContext(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := inventory.NewInventory(fs)

	// Set up inventory
	host := inv.GetOrCreateHost("web1")
	host.Variables["host_var"] = "host_value"

	group := inv.GetOrCreateGroup("webservers")
	group.Variables["group_var"] = "group_value"
	group.AddHost("web1")

	manager := NewManager(inv)
	manager.SetExtraVar("extra_var", "extra_value")

	ctx, err := manager.CreateHostContext("web1")
	if err != nil {
		t.Fatalf("Failed to create host context: %v", err)
	}

	// Test that host variables were added
	value, exists := ctx.GetVariable("host_var")
	if !exists {
		t.Error("Expected host variable to exist")
	} else if value != "host_value" {
		t.Errorf("Expected 'host_value', got '%v'", value)
	}

	// Test that group variables were added
	value, exists = ctx.GetVariable("group_var")
	if !exists {
		t.Error("Expected group variable to exist")
	} else if value != "group_value" {
		t.Errorf("Expected 'group_value', got '%v'", value)
	}

	// Test that extra variables were added
	value, exists = ctx.GetVariable("extra_var")
	if !exists {
		t.Error("Expected extra variable to exist")
	} else if value != "extra_value" {
		t.Errorf("Expected 'extra_value', got '%v'", value)
	}

	// Test that facts were added
	value, exists = ctx.GetFact("ansible_hostname")
	if !exists {
		t.Error("Expected ansible_hostname fact to exist")
	} else if value != "web1" {
		t.Errorf("Expected 'web1', got '%v'", value)
	}
}

func TestManager_TemplateString(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := inventory.NewInventory(fs)
	manager := NewManager(inv)

	ctx := NewContext()
	ctx.SetVariable("name", "world", PrecedenceTaskVars, "test")
	ctx.SetVariable("count", 42, PrecedenceTaskVars, "test")

	// Test simple variable substitution
	result, err := manager.TemplateString("Hello {{ name }}!", ctx)
	if err != nil {
		t.Fatalf("Template failed: %v", err)
	}

	expected := "Hello world!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}

	// Test with number
	result, err = manager.TemplateString("Count: {{ count }}", ctx)
	if err != nil {
		t.Fatalf("Template failed: %v", err)
	}

	expected = "Count: 42"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestManager_TemplateValue(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := inventory.NewInventory(fs)
	manager := NewManager(inv)

	ctx := NewContext()
	ctx.SetVariable("name", "world", PrecedenceTaskVars, "test")
	ctx.SetVariable("port", 8080, PrecedenceTaskVars, "test")

	tests := []struct {
		name     string
		input    interface{}
		expected interface{}
	}{
		{
			name:     "simple string",
			input:    "Hello {{ name }}!",
			expected: "Hello world!",
		},
		{
			name:     "non-template string",
			input:    "plain text",
			expected: "plain text",
		},
		{
			name:     "number",
			input:    42,
			expected: 42,
		},
		{
			name: "map with templates",
			input: map[string]interface{}{
				"greeting": "Hello {{ name }}!",
				"port":     "{{ port }}",
				"static":   "unchanged",
			},
			expected: map[string]interface{}{
				"greeting": "Hello world!",
				"port":     "8080",
				"static":   "unchanged",
			},
		},
		{
			name: "array with templates",
			input: []interface{}{
				"Hello {{ name }}!",
				42,
				"Port: {{ port }}",
			},
			expected: []interface{}{
				"Hello world!",
				42,
				"Port: 8080",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := manager.TemplateValue(test.input, ctx)
			if err != nil {
				t.Fatalf("Template failed: %v", err)
			}

			if !deepEqual(result, test.expected) {
				t.Errorf("Expected %v, got %v", test.expected, result)
			}
		})
	}
}

func TestContext_MergeContext(t *testing.T) {
	ctx1 := NewContext()
	ctx1.SetVariable("var1", "value1", PrecedenceGroupVars, "group")
	ctx1.SetVariable("var2", "old_value", PrecedenceGroupVars, "group")
	ctx1.SetFact("fact1", "fact_value1")
	ctx1.SetHostvar("host1", "hostvar1", "hostvalue1")

	ctx2 := NewContext()
	ctx2.SetVariable("var2", "new_value", PrecedenceExtraVars, "extra") // Higher precedence
	ctx2.SetVariable("var3", "value3", PrecedenceTaskVars, "task")
	ctx2.SetFact("fact2", "fact_value2")
	ctx2.SetHostvar("host1", "hostvar2", "hostvalue2")
	ctx2.SetHostvar("host2", "hostvar1", "hostvalue3")

	ctx1.MergeContext(ctx2)

	// Test variable merging with precedence
	value, exists := ctx1.GetVariable("var1")
	if !exists || value != "value1" {
		t.Errorf("Expected var1='value1', got %v (exists: %v)", value, exists)
	}

	value, exists = ctx1.GetVariable("var2")
	if !exists || value != "new_value" {
		t.Errorf("Expected var2='new_value' (higher precedence), got %v (exists: %v)", value, exists)
	}

	value, exists = ctx1.GetVariable("var3")
	if !exists || value != "value3" {
		t.Errorf("Expected var3='value3', got %v (exists: %v)", value, exists)
	}

	// Test fact merging
	value, exists = ctx1.GetFact("fact1")
	if !exists || value != "fact_value1" {
		t.Errorf("Expected fact1='fact_value1', got %v (exists: %v)", value, exists)
	}

	value, exists = ctx1.GetFact("fact2")
	if !exists || value != "fact_value2" {
		t.Errorf("Expected fact2='fact_value2', got %v (exists: %v)", value, exists)
	}

	// Test hostvar merging
	value, exists = ctx1.GetHostvar("host1", "hostvar1")
	if !exists || value != "hostvalue1" {
		t.Errorf("Expected host1.hostvar1='hostvalue1', got %v (exists: %v)", value, exists)
	}

	value, exists = ctx1.GetHostvar("host1", "hostvar2")
	if !exists || value != "hostvalue2" {
		t.Errorf("Expected host1.hostvar2='hostvalue2', got %v (exists: %v)", value, exists)
	}

	value, exists = ctx1.GetHostvar("host2", "hostvar1")
	if !exists || value != "hostvalue3" {
		t.Errorf("Expected host2.hostvar1='hostvalue3', got %v (exists: %v)", value, exists)
	}
}

func TestConvertToTypedValue(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected interface{}
	}{
		{"true", true},
		{"false", false},
		{"yes", true},
		{"no", false},
		{"on", true},
		{"off", false},
		{"True", true},
		{"FALSE", false},
		{"42", 42},
		{"0", 0},
		{"-123", -123},
		{"3.14", 3.14},
		{"0.0", 0.0},
		{"hello", "hello"},
		{"", ""},
		{123, 123}, // Non-string input should be returned as-is
	}

	for _, test := range tests {
		result := ConvertToTypedValue(test.input)
		if result != test.expected {
			t.Errorf("ConvertToTypedValue(%v) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestLoadVarsFromYAML(t *testing.T) {
	yamlData := `
name: John
age: 30
enabled: true
height: 5.9
tags:
  - developer
  - golang
config:
  debug: false
  port: 8080
`

	vars, err := LoadVarsFromYAML([]byte(yamlData))
	if err != nil {
		t.Fatalf("Failed to load vars from YAML: %v", err)
	}

	// Test string value
	if vars["name"] != "John" {
		t.Errorf("Expected name='John', got %v", vars["name"])
	}

	// Test integer value
	if vars["age"] != 30 {
		t.Errorf("Expected age=30, got %v", vars["age"])
	}

	// Test boolean value
	if vars["enabled"] != true {
		t.Errorf("Expected enabled=true, got %v", vars["enabled"])
	}

	// Test float value
	if vars["height"] != 5.9 {
		t.Errorf("Expected height=5.9, got %v", vars["height"])
	}
}

func TestContext_DotNotationGet(t *testing.T) {
	ctx := NewContext()

	// Set up nested data structure
	user := map[string]interface{}{
		"name": "John",
		"details": map[string]interface{}{
			"age":   30,
			"email": "john@example.com",
		},
	}
	ctx.SetVariable("user", user, PrecedenceTaskVars, "test")

	// Test simple dot notation
	value, exists := ctx.DotNotationGet("user.name")
	if !exists {
		t.Fatal("Expected user.name to exist")
	}
	if value != "John" {
		t.Errorf("Expected 'John', got %v", value)
	}

	// Test nested dot notation
	value, exists = ctx.DotNotationGet("user.details.age")
	if !exists {
		t.Fatal("Expected user.details.age to exist")
	}
	if value != 30 {
		t.Errorf("Expected 30, got %v", value)
	}

	// Test non-existent path
	_, exists = ctx.DotNotationGet("user.nonexistent")
	if exists {
		t.Error("Expected user.nonexistent to not exist")
	}

	// Test non-existent root
	_, exists = ctx.DotNotationGet("nonexistent.field")
	if exists {
		t.Error("Expected nonexistent.field to not exist")
	}
}

func TestContext_DotNotationSet(t *testing.T) {
	ctx := NewContext()

	// Test setting simple value
	err := ctx.DotNotationSet("simple", "value", PrecedenceTaskVars, "test")
	if err != nil {
		t.Fatalf("Failed to set simple value: %v", err)
	}

	value, exists := ctx.GetVariable("simple")
	if !exists || value != "value" {
		t.Errorf("Expected simple='value', got %v (exists: %v)", value, exists)
	}

	// Test setting nested value
	err = ctx.DotNotationSet("user.name", "John", PrecedenceTaskVars, "test")
	if err != nil {
		t.Fatalf("Failed to set nested value: %v", err)
	}

	value, exists = ctx.DotNotationGet("user.name")
	if !exists || value != "John" {
		t.Errorf("Expected user.name='John', got %v (exists: %v)", value, exists)
	}

	// Test setting deeper nested value
	err = ctx.DotNotationSet("user.details.age", 30, PrecedenceTaskVars, "test")
	if err != nil {
		t.Fatalf("Failed to set deeper nested value: %v", err)
	}

	value, exists = ctx.DotNotationGet("user.details.age")
	if !exists || value != 30 {
		t.Errorf("Expected user.details.age=30, got %v (exists: %v)", value, exists)
	}
}

func TestContext_Clone(t *testing.T) {
	ctx := NewContext()
	ctx.SetVariable("var1", "value1", PrecedenceTaskVars, "test")
	ctx.SetFact("fact1", "factvalue1")
	ctx.SetHostvar("host1", "hostvar1", "hostvalue1")
	ctx.Groups["group1"] = []string{"host1", "host2"}

	clone := ctx.Clone()

	// Test that values are copied
	value, exists := clone.GetVariable("var1")
	if !exists || value != "value1" {
		t.Errorf("Expected cloned var1='value1', got %v (exists: %v)", value, exists)
	}

	value, exists = clone.GetFact("fact1")
	if !exists || value != "factvalue1" {
		t.Errorf("Expected cloned fact1='factvalue1', got %v (exists: %v)", value, exists)
	}

	value, exists = clone.GetHostvar("host1", "hostvar1")
	if !exists || value != "hostvalue1" {
		t.Errorf("Expected cloned hostvar='hostvalue1', got %v (exists: %v)", value, exists)
	}

	if len(clone.Groups["group1"]) != 2 {
		t.Errorf("Expected 2 hosts in cloned group1, got %d", len(clone.Groups["group1"]))
	}

	// Test that clone is independent
	ctx.SetVariable("var1", "modified", PrecedenceExtraVars, "test")
	value, exists = clone.GetVariable("var1")
	if !exists || value != "value1" {
		t.Errorf("Expected cloned var1 to remain 'value1', got %v (exists: %v)", value, exists)
	}
}

func TestContext_FilterVariables(t *testing.T) {
	ctx := NewContext()
	ctx.SetVariable("ansible_hostname", "web1", PrecedenceTaskVars, "test")
	ctx.SetVariable("ansible_port", 22, PrecedenceTaskVars, "test")
	ctx.SetVariable("app_name", "myapp", PrecedenceTaskVars, "test")
	ctx.SetVariable("app_version", "1.0", PrecedenceTaskVars, "test")
	ctx.SetVariable("database_host", "db1", PrecedenceTaskVars, "test")

	// Test prefix matching
	ansibleVars := ctx.FilterVariables("ansible_*")
	if len(ansibleVars) != 2 {
		t.Errorf("Expected 2 ansible_* variables, got %d", len(ansibleVars))
	}

	if ansibleVars["ansible_hostname"] != "web1" {
		t.Errorf("Expected ansible_hostname='web1', got %v", ansibleVars["ansible_hostname"])
	}

	// Test suffix matching
	appVars := ctx.FilterVariables("app_*")
	if len(appVars) != 2 {
		t.Errorf("Expected 2 app_* variables, got %d", len(appVars))
	}

	// Test exact match
	exactVar := ctx.FilterVariables("database_host")
	if len(exactVar) != 1 {
		t.Errorf("Expected 1 exact match, got %d", len(exactVar))
	}

	// Test wildcard match
	allVars := ctx.FilterVariables("*")
	if len(allVars) != 5 {
		t.Errorf("Expected 5 variables with wildcard, got %d", len(allVars))
	}
}

// Helper function to compare complex data structures
func deepEqual(a, b interface{}) bool {
	switch va := a.(type) {
	case map[string]interface{}:
		vb, ok := b.(map[string]interface{})
		if !ok || len(va) != len(vb) {
			return false
		}
		for key, valA := range va {
			valB, exists := vb[key]
			if !exists || !deepEqual(valA, valB) {
				return false
			}
		}
		return true
	case []interface{}:
		vb, ok := b.([]interface{})
		if !ok || len(va) != len(vb) {
			return false
		}
		for i, valA := range va {
			if !deepEqual(valA, vb[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}