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
	"fmt"
	"os"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// FileActionPlugin implements the file action plugin
type FileActionPlugin struct {
	*BaseActionPlugin
}

// NewFileActionPlugin creates a new file action plugin
func NewFileActionPlugin() *FileActionPlugin {
	return &FileActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"file",
			"Manage files and file properties",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Run executes the file action plugin
func (a *FileActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	path := GetArgString(args, "path", "")
	state := GetArgString(args, "state", "file")

	// Validate arguments
	if path == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "path is required",
		}, nil
	}

	// Validate state
	validStates := []string{"file", "directory", "link", "hard", "touch", "absent"}
	if err := ValidateChoices(args, "state", validStates); err != nil {
		return &plugins.ActionResult{
			Failed:  true,
			Message: err.Error(),
		}, nil
	}

	// Check current state
	_, pathExists := a.checkPath(path)
	changed := false

	// Check mode handling
	if IsCheckMode(actionCtx) {
		return a.handleCheckMode(path, state, pathExists)
	}

	// Handle different states
	switch state {
	case "absent":
		if pathExists {
			if err := os.RemoveAll(path); err != nil {
				return &plugins.ActionResult{
					Failed:  true,
					Message: fmt.Sprintf("Failed to remove %s: %v", path, err),
				}, nil
			}
			changed = true
		}

	case "touch":
		if !pathExists {
			if err := a.touchFile(path); err != nil {
				return &plugins.ActionResult{
					Failed:  true,
					Message: fmt.Sprintf("Failed to touch %s: %v", path, err),
				}, nil
			}
			changed = true
		}

	case "directory":
		if !pathExists {
			if err := os.MkdirAll(path, 0755); err != nil {
				return &plugins.ActionResult{
					Failed:  true,
					Message: fmt.Sprintf("Failed to create directory %s: %v", path, err),
				}, nil
			}
			changed = true
		}

	case "file":
		if !pathExists {
			return &plugins.ActionResult{
				Failed:  true,
				Message: fmt.Sprintf("File %s does not exist", path),
			}, nil
		}
	}

	result := &plugins.ActionResult{
		Changed: changed,
		Results: make(map[string]interface{}),
	}

	result.Results["path"] = path
	result.Results["state"] = state

	if changed {
		result.Message = fmt.Sprintf("File %s %s", path, state)
	} else {
		result.Message = fmt.Sprintf("File %s already in desired state", path)
	}

	return result, nil
}

// checkPath checks if a path exists and returns info
func (a *FileActionPlugin) checkPath(path string) (os.FileInfo, bool) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	return stat, true
}

// touchFile creates an empty file or updates timestamp
func (a *FileActionPlugin) touchFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

// handleCheckMode returns appropriate result for check mode
func (a *FileActionPlugin) handleCheckMode(path, state string, pathExists bool) (*plugins.ActionResult, error) {
	result := &plugins.ActionResult{
		Results: make(map[string]interface{}),
	}

	result.Results["path"] = path
	result.Results["state"] = state

	switch state {
	case "absent":
		if pathExists {
			result.Changed = true
			result.Message = "Would remove " + path
		} else {
			result.Message = "Path already absent"
		}

	case "touch", "file":
		if !pathExists {
			if state == "touch" {
				result.Changed = true
				result.Message = "Would create file " + path
			} else {
				result.Failed = true
				result.Message = "File does not exist: " + path
			}
		} else {
			result.Message = "File already exists"
		}

	case "directory":
		if !pathExists {
			result.Changed = true
			result.Message = "Would create directory " + path
		} else {
			result.Message = "Directory already exists"
		}
	}

	return result, nil
}