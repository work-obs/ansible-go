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

package modules

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/work-obs/ansible-go/pkg/plugins"
)

// BaseModule provides common functionality for all modules
type BaseModule struct {
	name        string
	description string
	version     string
	author      string
}

// NewBaseModule creates a new base module
func NewBaseModule(name, description, version, author string) *BaseModule {
	return &BaseModule{
		name:        name,
		description: description,
		version:     version,
		author:      author,
	}
}

// Name returns the module name
func (m *BaseModule) Name() string {
	return m.name
}

// Type returns the plugin type
func (m *BaseModule) Type() plugins.PluginType {
	return plugins.PluginTypeModule
}

// GetInfo returns plugin information
func (m *BaseModule) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        m.name,
		Type:        plugins.PluginTypeModule,
		Description: m.description,
		Version:     m.version,
		Author:      []string{m.author},
	}
}

// ModuleResult represents the result of a module execution
type ModuleResult struct {
	Changed bool                   `json:"changed"`
	Failed  bool                   `json:"failed,omitempty"`
	Msg     string                 `json:"msg,omitempty"`
	RC      int                    `json:"rc,omitempty"`
	Stdout  string                 `json:"stdout,omitempty"`
	Stderr  string                 `json:"stderr,omitempty"`
	Results map[string]interface{} `json:"results,omitempty"`
}

// ToMap converts ModuleResult to map[string]interface{}
func (r *ModuleResult) ToMap() map[string]interface{} {
	result := map[string]interface{}{
		"changed": r.Changed,
	}

	if r.Failed {
		result["failed"] = r.Failed
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
	if r.Results != nil {
		for k, v := range r.Results {
			result[k] = v
		}
	}

	return result
}

// ExecutableModule defines the interface that all executable modules must implement
type ExecutableModule interface {
	plugins.ExecutablePlugin
	Run(ctx context.Context, args map[string]interface{}, config *config.Config) (*ModuleResult, error)
}

// CommonModuleArgs defines common arguments for modules
type CommonModuleArgs struct {
	CheckMode bool `json:"_check_mode,omitempty"`
	DiffMode  bool `json:"_diff_mode,omitempty"`
}

// ValidateArgs validates common module arguments
func (m *BaseModule) ValidateArgs(args map[string]interface{}) error {
	// Basic validation - check if required arguments are present
	// This is a simplified validation - real implementation would be more comprehensive
	return nil
}

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

// IsCheckMode determines if the module is running in check mode
func IsCheckMode(args map[string]interface{}) bool {
	return GetArgBool(args, "_ansible_check_mode", false) || GetArgBool(args, "check_mode", false)
}

// IsDiffMode determines if the module is running in diff mode
func IsDiffMode(args map[string]interface{}) bool {
	return GetArgBool(args, "_ansible_diff", false) || GetArgBool(args, "diff", false)
}

// FailResult creates a failed module result
func FailResult(msg string, rc int) *ModuleResult {
	return &ModuleResult{
		Failed:  true,
		Msg:     msg,
		RC:      rc,
		Changed: false,
	}
}

// OkResult creates a successful module result
func OkResult(changed bool, msg string) *ModuleResult {
	return &ModuleResult{
		Changed: changed,
		Msg:     msg,
		Failed:  false,
	}
}

// ChangedResult creates a successful changed module result
func ChangedResult(msg string) *ModuleResult {
	return &ModuleResult{
		Changed: true,
		Msg:     msg,
		Failed:  false,
	}
}

// UnchangedResult creates a successful unchanged module result
func UnchangedResult(msg string) *ModuleResult {
	return &ModuleResult{
		Changed: false,
		Msg:     msg,
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

// RunModule is a wrapper that converts ExecutableModule.Run to ExecutablePlugin.Execute
func RunModule(m ExecutableModule, ctx context.Context, moduleCtx *plugins.ModuleContext) (map[string]interface{}, error) {
	// Type assert the config
	config, ok := moduleCtx.Config.(*config.Config)
	if !ok {
		return nil, fmt.Errorf("invalid config type")
	}

	result, err := m.Run(ctx, moduleCtx.Args, config)
	if err != nil {
		return nil, err
	}
	return result.ToMap(), nil
}