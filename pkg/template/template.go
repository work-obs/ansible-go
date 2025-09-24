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
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unicode"
)

// Engine provides Jinja2-compatible template rendering
type Engine struct {
	functions map[string]interface{}
	filters   map[string]interface{}
	tests     map[string]interface{}
}

// Context holds the template rendering context
type Context struct {
	Variables map[string]interface{}
	Hostvars  map[string]map[string]interface{}
	Groups    map[string][]string
	Inventory map[string]interface{}
	Facts     map[string]interface{}
}

// NewEngine creates a new template engine
func NewEngine() *Engine {
	engine := &Engine{
		functions: make(map[string]interface{}),
		filters:   make(map[string]interface{}),
		tests:     make(map[string]interface{}),
	}

	// Register default functions, filters, and tests
	engine.registerDefaults()
	return engine
}

// Render renders a template string with the given context
func (e *Engine) Render(templateStr string, ctx *Context) (string, error) {
	// Convert Jinja2-style template to Go template
	goTemplate, err := e.convertJinja2ToGoTemplate(templateStr)
	if err != nil {
		return "", fmt.Errorf("failed to convert template: %w", err)
	}

	// Create Go template
	tmpl, err := template.New("ansible").
		Funcs(e.createFuncMap(ctx)).
		Parse(goTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Prepare template data
	data := e.prepareTemplateData(ctx)

	// Execute template
	var result strings.Builder
	err = tmpl.Execute(&result, data)
	if err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return result.String(), nil
}

// RenderBool renders a template and returns a boolean result
func (e *Engine) RenderBool(templateStr string, ctx *Context) (bool, error) {
	result, err := e.Render(templateStr, ctx)
	if err != nil {
		return false, err
	}

	return e.toBool(strings.TrimSpace(result)), nil
}

// convertJinja2ToGoTemplate converts Jinja2 template syntax to Go template syntax
func (e *Engine) convertJinja2ToGoTemplate(templateStr string) (string, error) {
	result := templateStr

	// Convert variable expressions: {{ var }} -> {{.Variables.var}}
	varRegex := regexp.MustCompile(`\{\{\s*([^}]+)\s*\}\}`)
	result = varRegex.ReplaceAllStringFunc(result, func(match string) string {
		expr := strings.TrimSpace(match[2 : len(match)-2])
		goExpr := e.convertExpression(expr)
		return "{{" + goExpr + "}}"
	})

	// Convert control structures: {% ... %} -> {{...}}
	controlRegex := regexp.MustCompile(`\{%\s*([^%]+)\s*%\}`)
	result = controlRegex.ReplaceAllStringFunc(result, func(match string) string {
		expr := strings.TrimSpace(match[2 : len(match)-2])
		return e.convertControlStructure(expr)
	})

	return result, nil
}

// convertExpression converts a Jinja2 expression to Go template expression
func (e *Engine) convertExpression(expr string) string {
	expr = strings.TrimSpace(expr)

	// Handle filters: var | filter -> (filter .Variables.var)
	if strings.Contains(expr, "|") {
		return e.convertFilter(expr)
	}

	// Handle tests: var is test -> (test .Variables.var)
	if strings.Contains(expr, " is ") {
		return e.convertTest(expr)
	}

	// Handle attribute access: var.attr -> .Variables.var.attr
	if strings.Contains(expr, ".") {
		return e.convertAttributeAccess(expr)
	}

	// Handle array/dict access: var[key] -> (index .Variables.var key)
	if strings.Contains(expr, "[") {
		return e.convertArrayAccess(expr)
	}

	// Handle arithmetic and comparison operators
	expr = e.convertOperators(expr)

	// Simple variable: var -> .Variables.var
	if e.isSimpleVariable(expr) {
		return ".Variables." + expr
	}

	return expr
}

// convertFilter converts Jinja2 filter syntax to Go template function calls
func (e *Engine) convertFilter(expr string) string {
	parts := strings.Split(expr, "|")
	if len(parts) < 2 {
		return expr
	}

	variable := strings.TrimSpace(parts[0])
	filters := parts[1:]

	result := e.convertExpression(variable)

	for _, filter := range filters {
		filterName := strings.TrimSpace(filter)
		filterArgs := ""

		// Handle filters with arguments: filter(arg1, arg2)
		if strings.Contains(filterName, "(") {
			parenIndex := strings.Index(filterName, "(")
			filterArgs = filterName[parenIndex+1 : len(filterName)-1]
			filterName = filterName[:parenIndex]
		}

		if filterArgs != "" {
			result = fmt.Sprintf("(%s %s %s)", filterName, result, filterArgs)
		} else {
			result = fmt.Sprintf("(%s %s)", filterName, result)
		}
	}

	return result
}

// convertTest converts Jinja2 test syntax to Go template function calls
func (e *Engine) convertTest(expr string) string {
	parts := strings.Split(expr, " is ")
	if len(parts) != 2 {
		return expr
	}

	variable := strings.TrimSpace(parts[0])
	test := strings.TrimSpace(parts[1])

	// Handle negation: var is not test -> (not (test .Variables.var))
	negate := false
	if strings.HasPrefix(test, "not ") {
		negate = true
		test = strings.TrimSpace(test[4:])
	}

	variableExpr := e.convertExpression(variable)
	result := fmt.Sprintf("(%s %s)", test, variableExpr)

	if negate {
		result = fmt.Sprintf("(not %s)", result)
	}

	return result
}

// convertAttributeAccess converts attribute access syntax
func (e *Engine) convertAttributeAccess(expr string) string {
	// Handle hostvars, groups, etc.
	if strings.HasPrefix(expr, "hostvars") {
		return ".Hostvars" + expr[8:]
	}
	if strings.HasPrefix(expr, "groups") {
		return ".Groups" + expr[6:]
	}
	if strings.HasPrefix(expr, "inventory") {
		return ".Inventory" + expr[9:]
	}
	if strings.HasPrefix(expr, "ansible_facts") {
		return ".Facts" + expr[13:]
	}

	return ".Variables." + expr
}

// convertArrayAccess converts array/dict access syntax
func (e *Engine) convertArrayAccess(expr string) string {
	// Simple implementation for var[key] -> (index .Variables.var key)
	bracketIndex := strings.Index(expr, "[")
	if bracketIndex == -1 {
		return expr
	}

	variable := expr[:bracketIndex]
	key := expr[bracketIndex+1 : len(expr)-1]

	variableExpr := e.convertExpression(variable)
	return fmt.Sprintf("(index %s %s)", variableExpr, key)
}

// convertOperators converts arithmetic and comparison operators
func (e *Engine) convertOperators(expr string) string {
	// Replace Jinja2 operators with Go template equivalents
	operators := map[string]string{
		" and ":  " && ",
		" or ":   " || ",
		" not ":  " ! ",
		" == ":   " eq ",
		" != ":   " ne ",
		" < ":    " lt ",
		" <= ":   " le ",
		" > ":    " gt ",
		" >= ":   " ge ",
		" in ":   " | contains ",
	}

	result := expr
	for jinja2Op, goOp := range operators {
		result = strings.ReplaceAll(result, jinja2Op, goOp)
	}

	return result
}

// convertControlStructure converts Jinja2 control structures
func (e *Engine) convertControlStructure(expr string) string {
	parts := strings.Fields(expr)
	if len(parts) == 0 {
		return ""
	}

	command := parts[0]

	switch command {
	case "if":
		condition := strings.Join(parts[1:], " ")
		goCondition := e.convertExpression(condition)
		return "{{if " + goCondition + "}}"

	case "elif":
		condition := strings.Join(parts[1:], " ")
		goCondition := e.convertExpression(condition)
		return "{{else if " + goCondition + "}}"

	case "else":
		return "{{else}}"

	case "endif":
		return "{{end}}"

	case "for":
		// Handle: for item in items -> {{range .Variables.items}}
		if len(parts) >= 4 && parts[2] == "in" {
			variable := parts[1]
			iterable := strings.Join(parts[3:], " ")
			iterableExpr := e.convertExpression(iterable)
			return fmt.Sprintf("{{range $%s := %s}}", variable, iterableExpr)
		}

	case "endfor":
		return "{{end}}"

	case "set":
		// Handle: set var = value -> {{$var := value}}
		if len(parts) >= 4 && parts[2] == "=" {
			variable := parts[1]
			value := strings.Join(parts[3:], " ")
			valueExpr := e.convertExpression(value)
			return fmt.Sprintf("{{$%s := %s}}", variable, valueExpr)
		}
	}

	return "{{/* Unsupported control structure: " + expr + " */}}"
}

// isSimpleVariable checks if a string is a simple variable name
func (e *Engine) isSimpleVariable(s string) bool {
	if len(s) == 0 {
		return false
	}

	// Must start with letter or underscore
	if !unicode.IsLetter(rune(s[0])) && s[0] != '_' {
		return false
	}

	// Rest must be letters, digits, or underscores
	for _, r := range s[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}

	return true
}

// createFuncMap creates a function map for Go templates
func (e *Engine) createFuncMap(ctx *Context) template.FuncMap {
	funcMap := template.FuncMap{
		// Arithmetic functions
		"add": func(a, b interface{}) interface{} {
			return e.add(a, b)
		},
		"sub": func(a, b interface{}) interface{} {
			return e.sub(a, b)
		},
		"mul": func(a, b interface{}) interface{} {
			return e.mul(a, b)
		},
		"div": func(a, b interface{}) interface{} {
			return e.div(a, b)
		},

		// String functions
		"upper": func(s interface{}) string {
			return strings.ToUpper(e.toString(s))
		},
		"lower": func(s interface{}) string {
			return strings.ToLower(e.toString(s))
		},
		"title": func(s interface{}) string {
			return strings.Title(e.toString(s))
		},
		"trim": func(s interface{}) string {
			return strings.TrimSpace(e.toString(s))
		},
		"replace": func(s, old, new interface{}) string {
			return strings.ReplaceAll(e.toString(s), e.toString(old), e.toString(new))
		},

		// List functions
		"length": func(v interface{}) int {
			return e.length(v)
		},
		"first": func(v interface{}) interface{} {
			return e.first(v)
		},
		"last": func(v interface{}) interface{} {
			return e.last(v)
		},
		"join": func(v interface{}, sep string) string {
			return e.join(v, sep)
		},

		// Dict functions
		"keys": func(v interface{}) []string {
			return e.keys(v)
		},
		"values": func(v interface{}) []interface{} {
			return e.values(v)
		},

		// Test functions
		"defined": func(v interface{}) bool {
			return v != nil
		},
		"undefined": func(v interface{}) bool {
			return v == nil
		},
		"none": func(v interface{}) bool {
			return v == nil
		},
		"string": func(v interface{}) bool {
			_, ok := v.(string)
			return ok
		},
		"number": func(v interface{}) bool {
			return e.isNumber(v)
		},
		"boolean": func(v interface{}) bool {
			_, ok := v.(bool)
			return ok
		},
		"list": func(v interface{}) bool {
			return e.isList(v)
		},
		"dict": func(v interface{}) bool {
			return e.isDict(v)
		},

		// Logic functions
		"not": func(v interface{}) bool {
			return !e.toBool(v)
		},

		// Ansible-specific functions
		"default": func(v, defaultVal interface{}) interface{} {
			if v == nil {
				return defaultVal
			}
			return v
		},
	}

	// Add custom functions
	for name, fn := range e.functions {
		funcMap[name] = fn
	}

	// Add custom filters
	for name, fn := range e.filters {
		funcMap[name] = fn
	}

	// Add custom tests
	for name, fn := range e.tests {
		funcMap[name] = fn
	}

	return funcMap
}

// prepareTemplateData prepares the data structure for template execution
func (e *Engine) prepareTemplateData(ctx *Context) map[string]interface{} {
	data := make(map[string]interface{})

	if ctx.Variables != nil {
		data["Variables"] = ctx.Variables
	} else {
		data["Variables"] = make(map[string]interface{})
	}

	if ctx.Hostvars != nil {
		data["Hostvars"] = ctx.Hostvars
	} else {
		data["Hostvars"] = make(map[string]map[string]interface{})
	}

	if ctx.Groups != nil {
		data["Groups"] = ctx.Groups
	} else {
		data["Groups"] = make(map[string][]string)
	}

	if ctx.Inventory != nil {
		data["Inventory"] = ctx.Inventory
	} else {
		data["Inventory"] = make(map[string]interface{})
	}

	if ctx.Facts != nil {
		data["Facts"] = ctx.Facts
	} else {
		data["Facts"] = make(map[string]interface{})
	}

	return data
}

// registerDefaults registers default functions, filters, and tests
func (e *Engine) registerDefaults() {
	// This can be extended with more complex default implementations
}

// Utility functions for type conversion and operations

func (e *Engine) toString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func (e *Engine) toBool(v interface{}) bool {
	if v == nil {
		return false
	}

	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val != "" && strings.ToLower(val) != "false" && val != "0"
	case int, int64, float64:
		return val != 0
	default:
		return true
	}
}

func (e *Engine) toInt(v interface{}) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return 0
}

func (e *Engine) toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0.0
}

func (e *Engine) add(a, b interface{}) interface{} {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	if aVal.Kind() == reflect.String || bVal.Kind() == reflect.String {
		return e.toString(a) + e.toString(b)
	}

	return e.toFloat(a) + e.toFloat(b)
}

func (e *Engine) sub(a, b interface{}) interface{} {
	return e.toFloat(a) - e.toFloat(b)
}

func (e *Engine) mul(a, b interface{}) interface{} {
	return e.toFloat(a) * e.toFloat(b)
}

func (e *Engine) div(a, b interface{}) interface{} {
	bFloat := e.toFloat(b)
	if bFloat == 0 {
		return 0
	}
	return e.toFloat(a) / bFloat
}

func (e *Engine) length(v interface{}) int {
	if v == nil {
		return 0
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return val.Len()
	}
	return 0
}

func (e *Engine) first(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Slice && val.Len() > 0 {
		return val.Index(0).Interface()
	}
	return nil
}

func (e *Engine) last(v interface{}) interface{} {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Slice && val.Len() > 0 {
		return val.Index(val.Len() - 1).Interface()
	}
	return nil
}

func (e *Engine) join(v interface{}, sep string) string {
	if v == nil {
		return ""
	}

	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Slice {
		return e.toString(v)
	}

	var parts []string
	for i := 0; i < val.Len(); i++ {
		parts = append(parts, e.toString(val.Index(i).Interface()))
	}

	return strings.Join(parts, sep)
}

func (e *Engine) keys(v interface{}) []string {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Map {
		return nil
	}

	var keys []string
	for _, key := range val.MapKeys() {
		keys = append(keys, e.toString(key.Interface()))
	}
	return keys
}

func (e *Engine) values(v interface{}) []interface{} {
	if v == nil {
		return nil
	}

	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Map {
		return nil
	}

	var values []interface{}
	for _, key := range val.MapKeys() {
		values = append(values, val.MapIndex(key).Interface())
	}
	return values
}

func (e *Engine) isNumber(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return true
	}
	return false
}

func (e *Engine) isList(v interface{}) bool {
	if v == nil {
		return false
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Slice || val.Kind() == reflect.Array
}

func (e *Engine) isDict(v interface{}) bool {
	if v == nil {
		return false
	}
	val := reflect.ValueOf(v)
	return val.Kind() == reflect.Map
}

// AddFunction adds a custom function to the template engine
func (e *Engine) AddFunction(name string, fn interface{}) {
	e.functions[name] = fn
}

// AddFilter adds a custom filter to the template engine
func (e *Engine) AddFilter(name string, fn interface{}) {
	e.filters[name] = fn
}

// AddTest adds a custom test to the template engine
func (e *Engine) AddTest(name string, fn interface{}) {
	e.tests[name] = fn
}