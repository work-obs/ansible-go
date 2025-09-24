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
	"path/filepath"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// CopyActionPlugin implements the copy action plugin
type CopyActionPlugin struct {
	*BaseActionPlugin
}

// NewCopyActionPlugin creates a new copy action plugin
func NewCopyActionPlugin() *CopyActionPlugin {
	return &CopyActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"copy",
			"Copy files to remote locations",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Run executes the copy action plugin
func (a *CopyActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	src := GetArgString(args, "src", "")
	content := GetArgString(args, "content", "")
	dest := GetArgString(args, "dest", "")

	// Validate arguments
	if dest == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "dest is required",
		}, nil
	}

	if src == "" && content == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "either src or content is required",
		}, nil
	}

	if src != "" && content != "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "src and content are mutually exclusive",
		}, nil
	}

	// Check mode handling
	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: true,
			Message: "Would copy file",
		}, nil
	}

	// For now, implement basic local copy (full implementation would handle remote copying)
	var err error
	changed := false

	if src != "" {
		// Copy from source file
		err = a.copyFile(src, dest)
		changed = true
	} else {
		// Write content to destination
		err = a.writeContent(content, dest)
		changed = true
	}

	if err != nil {
		return &plugins.ActionResult{
			Failed:  true,
			Message: fmt.Sprintf("Copy failed: %v", err),
		}, nil
	}

	result := &plugins.ActionResult{
		Changed: changed,
		Message: "File copied successfully",
		Results: make(map[string]interface{}),
	}

	result.Results["dest"] = dest
	if src != "" {
		result.Results["src"] = src
	}

	return result, nil
}

// copyFile copies a file from src to dest
func (a *CopyActionPlugin) copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	return os.WriteFile(dest, data, 0644)
}

// writeContent writes content to a file
func (a *CopyActionPlugin) writeContent(content, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	return os.WriteFile(dest, []byte(content), 0644)
}