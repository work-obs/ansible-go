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

package template

import (
	"strings"
	"testing"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine()

	if engine == nil {
		t.Fatal("Expected non-nil engine")
	}

	if engine.functions == nil {
		t.Error("Expected functions map to be initialized")
	}

	if engine.filters == nil {
		t.Error("Expected filters map to be initialized")
	}

	if engine.tests == nil {
		t.Error("Expected tests map to be initialized")
	}
}

func TestEngine_Render_SimpleVariable(t *testing.T) {
	engine := NewEngine()
	ctx := &Context{
		Variables: map[string]interface{}{
			"name": "world",
		},
	}

	template := "Hello {{ name }}!"
	result, err := engine.Render(template, ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := "Hello world!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestEngine_Render_AttributeAccess(t *testing.T) {
	engine := NewEngine()
	ctx := &Context{
		Variables: map[string]interface{}{
			"user": map[string]interface{}{
				"name": "john",
				"age":  30,
			},
		},
	}

	template := "User: {{ user.name }}, Age: {{ user.age }}"
	result, err := engine.Render(template, ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := "User: john, Age: 30"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestEngine_Render_Filter(t *testing.T) {
	engine := NewEngine()
	ctx := &Context{
		Variables: map[string]interface{}{
			"name": "world",
		},
	}

	template := "Hello {{ name | upper }}!"
	result, err := engine.Render(template, ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expected := "Hello WORLD!"
	if result != expected {
		t.Errorf("Expected '%s', got '%s'", expected, result)
	}
}

func TestEngine_Render_DefaultFilter(t *testing.T) {
	engine := NewEngine()
	ctx := &Context{
		Variables: map[string]interface{}{
			"defined_var": "value",
		},
	}

	template := "{{ undefined_var | default('default_value') }} and {{ defined_var | default('other') }}"
	result, err := engine.Render(template, ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Note: This is a simplified test - the actual implementation would need more complex logic
	// for proper Jinja2 compatibility
	if !contains(result, "default_value") || !contains(result, "value") {
		t.Errorf("Expected result to contain both default_value and value, got '%s'", result)
	}
}

func TestEngine_RenderBool(t *testing.T) {
	engine := NewEngine()
	ctx := &Context{
		Variables: map[string]interface{}{
			"enabled": true,
			"count":   5,
			"name":    "test",
		},
	}

	tests := []struct {
		template string
		expected bool
	}{
		{"{{ enabled }}", true},
		{"{{ count }}", true},  // Non-zero number should be true
		{"{{ name }}", true},   // Non-empty string should be true
		{"false", false},
		{"0", false},
		{"", false},
	}

	for _, test := range tests {
		result, err := engine.RenderBool(test.template, ctx)
		if err != nil {
			t.Errorf("Template '%s' failed with error: %v", test.template, err)
			continue
		}

		if result != test.expected {
			t.Errorf("Template '%s': expected %v, got %v", test.template, test.expected, result)
		}
	}
}

func TestEngine_ConvertExpression(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		input    string
		contains string // What the output should contain
	}{
		{"variable", ".Variables.variable"},
		{"hostvars.host1", ".Hostvars"},
		{"groups.webservers", ".Groups"},
		{"inventory_hostname", ".Variables.inventory_hostname"},
	}

	for _, test := range tests {
		result := engine.convertExpression(test.input)
		if !contains(result, test.contains) {
			t.Errorf("convertExpression('%s') = '%s', should contain '%s'",
				test.input, result, test.contains)
		}
	}
}

func TestEngine_IsSimpleVariable(t *testing.T) {
	engine := NewEngine()

	tests := []struct {
		input    string
		expected bool
	}{
		{"variable", true},
		{"var_name", true},
		{"_private", true},
		{"var123", true},
		{"123var", false},      // Can't start with number
		{"var.attr", false},    // Contains dot
		{"var[key]", false},    // Contains brackets
		{"", false},            // Empty string
		{"var name", false},    // Contains space
	}

	for _, test := range tests {
		result := engine.isSimpleVariable(test.input)
		if result != test.expected {
			t.Errorf("isSimpleVariable('%s') = %v, expected %v",
				test.input, result, test.expected)
		}
	}
}

func TestEngine_UtilityFunctions(t *testing.T) {
	engine := NewEngine()

	// Test toString
	if engine.toString("test") != "test" {
		t.Error("toString failed for string")
	}
	if engine.toString(123) != "123" {
		t.Error("toString failed for int")
	}
	if engine.toString(nil) != "" {
		t.Error("toString failed for nil")
	}

	// Test toBool
	if !engine.toBool(true) {
		t.Error("toBool failed for true")
	}
	if engine.toBool(false) {
		t.Error("toBool failed for false")
	}
	if engine.toBool("") {
		t.Error("toBool failed for empty string")
	}
	if !engine.toBool("test") {
		t.Error("toBool failed for non-empty string")
	}
	if engine.toBool(0) {
		t.Error("toBool failed for zero")
	}
	if !engine.toBool(1) {
		t.Error("toBool failed for non-zero")
	}

	// Test toInt
	if engine.toInt(123) != 123 {
		t.Error("toInt failed for int")
	}
	if engine.toInt("123") != 123 {
		t.Error("toInt failed for string")
	}
	if engine.toInt("invalid") != 0 {
		t.Error("toInt failed for invalid string")
	}

	// Test toFloat
	if engine.toFloat(123.5) != 123.5 {
		t.Error("toFloat failed for float")
	}
	if engine.toFloat(123) != 123.0 {
		t.Error("toFloat failed for int")
	}
	if engine.toFloat("123.5") != 123.5 {
		t.Error("toFloat failed for string")
	}
}

func TestEngine_ArithmeticOperations(t *testing.T) {
	engine := NewEngine()

	// Test add
	if engine.add(1, 2) != 3.0 {
		t.Error("add failed for numbers")
	}
	if engine.add("hello", " world") != "hello world" {
		t.Error("add failed for strings")
	}

	// Test sub
	if engine.sub(5, 3) != 2.0 {
		t.Error("sub failed")
	}

	// Test mul
	if engine.mul(3, 4) != 12.0 {
		t.Error("mul failed")
	}

	// Test div
	if engine.div(10, 2) != 5.0 {
		t.Error("div failed")
	}
	if engine.div(10, 0) != 0 {
		t.Error("div by zero should return 0")
	}
}

func TestEngine_CollectionOperations(t *testing.T) {
	engine := NewEngine()

	// Test length
	if engine.length("hello") != 5 {
		t.Error("length failed for string")
	}
	if engine.length([]int{1, 2, 3}) != 3 {
		t.Error("length failed for slice")
	}
	if engine.length(map[string]int{"a": 1, "b": 2}) != 2 {
		t.Error("length failed for map")
	}

	// Test first
	slice := []string{"first", "second", "third"}
	if engine.first(slice) != "first" {
		t.Error("first failed")
	}

	// Test last
	if engine.last(slice) != "third" {
		t.Error("last failed")
	}

	// Test join
	if engine.join(slice, ", ") != "first, second, third" {
		t.Error("join failed")
	}

	// Test keys
	testMap := map[string]int{"a": 1, "b": 2}
	keys := engine.keys(testMap)
	if len(keys) != 2 {
		t.Error("keys failed - wrong length")
	}

	// Test values
	values := engine.values(testMap)
	if len(values) != 2 {
		t.Error("values failed - wrong length")
	}
}

func TestEngine_TypeChecks(t *testing.T) {
	engine := NewEngine()

	// Test isNumber
	if !engine.isNumber(123) {
		t.Error("isNumber failed for int")
	}
	if !engine.isNumber(123.5) {
		t.Error("isNumber failed for float")
	}
	if engine.isNumber("123") {
		t.Error("isNumber should fail for string")
	}

	// Test isList
	if !engine.isList([]int{1, 2, 3}) {
		t.Error("isList failed for slice")
	}
	if engine.isList(map[string]int{"a": 1}) {
		t.Error("isList should fail for map")
	}

	// Test isDict
	if !engine.isDict(map[string]int{"a": 1}) {
		t.Error("isDict failed for map")
	}
	if engine.isDict([]int{1, 2, 3}) {
		t.Error("isDict should fail for slice")
	}
}

func TestEngine_AddCustomFunction(t *testing.T) {
	engine := NewEngine()

	// Add custom function
	engine.AddFunction("double", func(x int) int {
		return x * 2
	})

	// Verify it was added
	if _, exists := engine.functions["double"]; !exists {
		t.Error("Custom function was not added")
	}
}

func TestEngine_AddCustomFilter(t *testing.T) {
	engine := NewEngine()

	// Add custom filter
	engine.AddFilter("reverse", func(s string) string {
		runes := []rune(s)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes)
	})

	// Verify it was added
	if _, exists := engine.filters["reverse"]; !exists {
		t.Error("Custom filter was not added")
	}
}

func TestEngine_AddCustomTest(t *testing.T) {
	engine := NewEngine()

	// Add custom test
	engine.AddTest("even", func(x int) bool {
		return x%2 == 0
	})

	// Verify it was added
	if _, exists := engine.tests["even"]; !exists {
		t.Error("Custom test was not added")
	}
}

// Helper function
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}