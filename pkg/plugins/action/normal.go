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
	"context"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// NormalActionPlugin implements the normal/generic action plugin
// This is the default action plugin for most modules that don't have
// specialized action plugins
type NormalActionPlugin struct {
	*BaseActionPlugin
}

// NewNormalActionPlugin creates a new normal action plugin
func NewNormalActionPlugin() *NormalActionPlugin {
	return &NormalActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"normal",
			"Generic action plugin for standard modules",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Run executes the normal action plugin
func (a *NormalActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	// The normal action plugin typically:
	// 1. Validates arguments
	// 2. Handles check mode
	// 3. Transfers the module to the remote host (if needed)
	// 4. Executes the module on the remote host
	// 5. Processes the result

	// For now, we'll implement a basic version that simulates module execution
	// In a full implementation, this would handle module transfer and execution

	result := &plugins.ActionResult{
		Changed: false,
		Message: "Normal action plugin executed (placeholder implementation)",
		Results: make(map[string]interface{}),
	}

	// Add module information to results
	if actionCtx.TaskVars != nil {
		result.Results["task_vars"] = actionCtx.TaskVars
	}

	result.Results["module_args"] = actionCtx.Args

	// Check mode handling
	if IsCheckMode(actionCtx) {
		result.Changed = true
		result.Message = "Would execute module in check mode"
		return result, nil
	}

	// In a real implementation, this is where we would:
	// 1. Create a temporary directory on the remote host
	// 2. Transfer the module file
	// 3. Execute the module with the provided arguments
	// 4. Parse and return the module's JSON output

	// For demonstration, we'll simulate successful execution
	result.Results["simulated"] = true
	result.Results["module_execution"] = "completed"

	return result, nil
}