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
	"reflect"
	"strconv"
	"strings"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// BaseActionPlugin provides common functionality for all action plugins
type BaseActionPlugin struct {
	name        string
	description string
	version     string
	author      string
}

// NewBaseActionPlugin creates a new base action plugin
func NewBaseActionPlugin(name, description, version, author string) *BaseActionPlugin {
	return &BaseActionPlugin{
		name:        name,
		description: description,
		version:     version,
		author:      author,
	}
}

// Name returns the plugin name
func (a *BaseActionPlugin) Name() string {
	return a.name
}

// Type returns the plugin type
func (a *BaseActionPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeAction
}

// GetInfo returns plugin information
func (a *BaseActionPlugin) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        a.name,
		Type:        plugins.PluginTypeAction,
		Description: a.description,
		Version:     a.version,
		Author:      []string{a.author},
	}
}

// GetRequiredConnection returns the connection type needed (default: smart)
func (a *BaseActionPlugin) GetRequiredConnection() string {
	return "smart"
}

// ActionResult represents the result of an action plugin execution
type ActionResult struct {
	Changed   bool                   `json:"changed"`
	Failed    bool                   `json:"failed,omitempty"`
	Skipped   bool                   `json:"skipped,omitempty"`
	Msg       string                 `json:"msg,omitempty"`
	RC        int                    `json:"rc,omitempty"`
	Stdout    string                 `json:"stdout,omitempty"`
	Stderr    string                 `json:"stderr,omitempty"`
	Results   map[string]interface{} `json:"results,omitempty"`
	Warnings  []string               `json:"warnings,omitempty"`
	Diff      map[string]interface{} `json:"diff,omitempty"`
}

// ToMap converts ActionResult to map[string]interface{}
func (r *ActionResult) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"changed": r.Changed,
	}

	if r.Failed {
		result["failed"] = r.Failed
	}
	if r.Skipped {
		result["skipped"] = r.Skipped
	}
	if r.Msg != "" {
		result["msg"] = r.Msg
	}
	if r.RC != 0 {
		result["rc"] = r.RC
	}
	if r.Stdout != "" {
		result["stdout"] = r.Stdout
	}
	if r.Stderr != "" {
		result["stderr"] = r.Stderr
	}
	if len(r.Warnings) > 0 {
		result["warnings"] = r.Warnings
	}
	if len(r.Diff) > 0 {
		result["diff"] = r.Diff
	}

	// Merge Results into top level
	if r.Results != nil {
		for k, v := range r.Results {
			result[k] = v
		}
	}

	return result
}

// ToPluginsActionResult converts to plugins.ActionResult
func (r *ActionResult) ToPluginsActionResult() *plugins.ActionResult {
	return &plugins.ActionResult{
		Changed:  r.Changed,
		Failed:   r.Failed,
		Skipped:  r.Skipped,
		Message:  r.Msg,
		Results:  r.Results,
		Warnings: r.Warnings,
		Diff:     r.Diff,
	}
}

// Validation and argument helper functions (moved from modules package)

// GetArgString retrieves a string argument with default value
func GetArgString(args map[string]interface{}, key, defaultValue string) string {
	if val, exists := args[key]; exists {
		if str, ok := val.(string); ok {
			return str
		}
		return fmt.Sprintf("%v", val)
	}
	return defaultValue
}

// GetArgBool retrieves a boolean argument with default value
func GetArgBool(args map[string]interface{}, key string, defaultValue bool) bool {
	if val, exists := args[key]; exists {
		switch v := val.(type) {
		case bool:
			return v
		case string:
			return strings.ToLower(v) == "true" || v == "yes" || v == "1"
		case int:
			return v != 0
		case float64:
			return v != 0
		}
	}
	return defaultValue
}

// GetArgInt retrieves an integer argument with default value
func GetArgInt(args map[string]interface{}, key string, defaultValue int) int {
	if val, exists := args[key]; exists {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			if i, err := strconv.Atoi(v); err == nil {
				return i
			}
		}
	}
	return defaultValue
}

// GetArgStringSlice retrieves a string slice argument
func GetArgStringSlice(args map[string]interface{}, key string) []string {
	if val, exists := args[key]; exists {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, len(v))
			for i, item := range v {
				result[i] = fmt.Sprintf("%v", item)
			}
			return result
		case string:
			// Split comma-separated string
			return strings.Split(v, ",")
		}
	}
	return nil
}

// GetArgMap retrieves a map argument
func GetArgMap(args map[string]interface{}, key string) map[string]interface{} {
	if val, exists := args[key]; exists {
		if m, ok := val.(map[string]interface{}); ok {
			return m
		}
	}
	return nil
}

// IsCheckMode determines if the action is running in check mode
func IsCheckMode(actionCtx *plugins.ActionContext) bool {
	return actionCtx.PlayContext != nil && actionCtx.PlayContext.CheckMode
}

// IsDiffMode determines if the action is running in diff mode
func IsDiffMode(actionCtx *plugins.ActionContext) bool {
	return actionCtx.PlayContext != nil && actionCtx.PlayContext.DiffMode
}

// FailResult creates a failed action result
func FailResult(msg string, rc int) *ActionResult {
	return &ActionResult{
		Failed:  true,
		Msg:     msg,
		RC:      rc,
		Changed: false,
	}
}

// OkResult creates a successful action result
func OkResult(changed bool, msg string) *ActionResult {
	return &ActionResult{
		Changed: changed,
		Msg:     msg,
		Failed:  false,
	}
}

// ChangedResult creates a successful changed action result
func ChangedResult(msg string) *ActionResult {
	return &ActionResult{
		Changed: true,
		Msg:     msg,
		Failed:  false,
	}
}

// UnchangedResult creates a successful unchanged action result
func UnchangedResult(msg string) *ActionResult {
	return &ActionResult{
		Changed: false,
		Msg:     msg,
		Failed:  false,
	}
}

// SkippedResult creates a skipped action result
func SkippedResult(msg string) *ActionResult {
	return &ActionResult{
		Skipped: true,
		Msg:     msg,
		Changed: false,
		Failed:  false,
	}
}

// ValidateRequired validates that required arguments are present
func ValidateRequired(args map[string]interface{}, required []string) error {
	missing := make([]string, 0)
	for _, req := range required {
		if _, exists := args[req]; !exists {
			missing = append(missing, req)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required arguments: %s", strings.Join(missing, ", "))
	}
	return nil
}

// ValidateChoices validates that an argument value is in allowed choices
func ValidateChoices(args map[string]interface{}, arg string, choices []string) error {
	if val, exists := args[arg]; exists {
		strVal := fmt.Sprintf("%v", val)
		for _, choice := range choices {
			if strVal == choice {
				return nil
			}
		}
		return fmt.Errorf("invalid value '%s' for argument '%s'. Valid choices are: %s",
			strVal, arg, strings.Join(choices, ", "))
	}
	return nil
}

// ValidateType validates that an argument is of the expected type
func ValidateType(args map[string]interface{}, arg string, expectedType reflect.Type) error {
	if val, exists := args[arg]; exists {
		actualType := reflect.TypeOf(val)
		if actualType != expectedType {
			return fmt.Errorf("argument '%s' expected type %s, got %s",
				arg, expectedType.String(), actualType.String())
		}
	}
	return nil
}