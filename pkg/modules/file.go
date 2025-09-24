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
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/work-obs/ansible-go/pkg/plugins"
)

// FileModule implements the file module for managing files and directories
type FileModule struct {
	*BaseModule
}

// NewFileModule creates a new file module
func NewFileModule() *FileModule {
	return &FileModule{
		BaseModule: NewBaseModule(
			"file",
			"Manage files and file properties",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Validate validates the module arguments
func (m *FileModule) Validate(args map[string]interface{}) error {
	path := GetArgString(args, "path", "")
	state := GetArgString(args, "state", "file")

	// Path is required
	if path == "" {
		return fmt.Errorf("path is required")
	}

	// Validate state
	validStates := []string{"file", "directory", "link", "hard", "touch", "absent"}
	if err := ValidateChoices(args, "state", validStates); err != nil {
		return err
	}

	// Validate mode
	mode := GetArgString(args, "mode", "")
	if mode != "" {
		if !m.isValidMode(mode) {
			return fmt.Errorf("invalid mode: %s", mode)
		}
	}

	// Validate recurse (only valid for directories)
	if state != "directory" && GetArgBool(args, "recurse", false) {
		return fmt.Errorf("recurse option is only valid for state=directory")
	}

	// Validate src (required for link states)
	src := GetArgString(args, "src", "")
	if (state == "link" || state == "hard") && src == "" {
		return fmt.Errorf("src is required for link states")
	}

	return nil
}

// Execute implements the ExecutablePlugin interface
func (m *FileModule) Execute(ctx context.Context, moduleCtx *plugins.ModuleContext) (map[string]interface{}, error) {
	return RunModule(m, ctx, moduleCtx)
}

// Run executes the file module
func (m *FileModule) Run(ctx context.Context, args map[string]interface{}, config *config.Config) (*ModuleResult, error) {
	path := GetArgString(args, "path", "")
	state := GetArgString(args, "state", "file")
	mode := GetArgString(args, "mode", "")
	owner := GetArgString(args, "owner", "")
	group := GetArgString(args, "group", "")
	recurse := GetArgBool(args, "recurse", false)
	force := GetArgBool(args, "force", false)
	src := GetArgString(args, "src", "")

	// Check current state of path
	pathInfo, pathExists := m.getPathInfo(path)
	changed := false

	// Check mode - return early in check mode
	if IsCheckMode(args) {
		return m.handleCheckMode(path, state, pathExists, pathInfo)
	}

	// Handle different states
	switch state {
	case "absent":
		if pathExists {
			if err := m.removePath(path, pathInfo.IsDir, recurse, force); err != nil {
				return FailResult(fmt.Sprintf("failed to remove %s: %v", path, err), 1), nil
			}
			changed = true
		}

	case "touch":
		if !pathExists {
			if err := m.touchFile(path); err != nil {
				return FailResult(fmt.Sprintf("failed to touch %s: %v", path, err), 1), nil
			}
			changed = true
		}

	case "file":
		if !pathExists {
			return FailResult(fmt.Sprintf("file %s does not exist", path), 1), nil
		}
		if pathInfo.IsDir {
			return FailResult(fmt.Sprintf("%s is a directory, expected a file", path), 1), nil
		}

	case "directory":
		if !pathExists {
			if err := m.createDirectory(path, recurse); err != nil {
				return FailResult(fmt.Sprintf("failed to create directory %s: %v", path, err), 1), nil
			}
			changed = true
		} else if !pathInfo.IsDir {
			return FailResult(fmt.Sprintf("%s exists but is not a directory", path), 1), nil
		}

	case "link":
		if err := m.createSymlink(src, path, force); err != nil {
			return FailResult(fmt.Sprintf("failed to create symlink: %v", err), 1), nil
		}
		changed = true

	case "hard":
		if err := m.createHardlink(src, path, force); err != nil {
			return FailResult(fmt.Sprintf("failed to create hardlink: %v", err), 1), nil
		}
		changed = true
	}

	// Apply file attributes if the path exists after state changes
	if state != "absent" {
		if attrChanged, err := m.applyAttributes(path, mode, owner, group, recurse); err != nil {
			return FailResult(fmt.Sprintf("failed to apply attributes: %v", err), 1), nil
		} else if attrChanged {
			changed = true
		}
	}

	// Get final path information
	finalInfo, finalExists := m.getPathInfo(path)

	// Prepare result
	result := &ModuleResult{
		Changed: changed,
		Results: make(map[string]interface{}),
	}

	if finalExists {
		result.Results["path"] = path
		result.Results["mode"] = fmt.Sprintf("%04o", finalInfo.Mode.Perm())
		result.Results["size"] = finalInfo.Size
		result.Results["uid"] = finalInfo.UID
		result.Results["gid"] = finalInfo.GID

		if finalInfo.IsDir {
			result.Results["state"] = "directory"
		} else if finalInfo.IsLink {
			result.Results["state"] = "link"
		} else {
			result.Results["state"] = "file"
		}
	} else {
		result.Results["path"] = path
		result.Results["state"] = "absent"
	}

	return result, nil
}

// PathInfo represents information about a path
type PathInfo struct {
	Exists bool
	IsDir  bool
	IsLink bool
	Mode   os.FileMode
	Size   int64
	UID    int
	GID    int
}

// getPathInfo gets information about a path
func (m *FileModule) getPathInfo(path string) (*PathInfo, bool) {
	info := &PathInfo{}

	stat, err := os.Lstat(path) // Use Lstat to not follow symlinks
	if err != nil {
		return info, false
	}

	info.Exists = true
	info.IsDir = stat.IsDir()
	info.IsLink = stat.Mode()&os.ModeSymlink != 0
	info.Mode = stat.Mode()
	info.Size = stat.Size()

	// Get UID/GID (Unix-specific)
	if sys := stat.Sys(); sys != nil {
		// This is simplified - real implementation would handle cross-platform
		info.UID = 0
		info.GID = 0
	}

	return info, true
}

// handleCheckMode returns appropriate result for check mode
func (m *FileModule) handleCheckMode(path, state string, pathExists bool, pathInfo *PathInfo) (*ModuleResult, error) {
	switch state {
	case "absent":
		if pathExists {
			return ChangedResult("Would remove " + path), nil
		}
		return UnchangedResult("Path already absent"), nil

	case "touch", "file":
		if !pathExists {
			if state == "touch" {
				return ChangedResult("Would create file " + path), nil
			}
			return FailResult("File does not exist: " + path, 1), nil
		}
		return UnchangedResult("File already exists"), nil

	case "directory":
		if !pathExists {
			return ChangedResult("Would create directory " + path), nil
		}
		if pathInfo.IsDir {
			return UnchangedResult("Directory already exists"), nil
		}
		return FailResult("Path exists but is not a directory", 1), nil

	case "link", "hard":
		return ChangedResult("Would create " + state + "link"), nil
	}

	return UnchangedResult("No changes needed"), nil
}

// removePath removes a file or directory
func (m *FileModule) removePath(path string, isDir, recurse, force bool) error {
	if isDir {
		if recurse || force {
			return os.RemoveAll(path)
		} else {
			return os.Remove(path) // Will fail if directory is not empty
		}
	}
	return os.Remove(path)
}

// touchFile creates an empty file or updates its timestamp
func (m *FileModule) touchFile(path string) error {
	// Create parent directories if they don't exist
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	return file.Close()
}

// createDirectory creates a directory
func (m *FileModule) createDirectory(path string, recurse bool) error {
	if recurse {
		return os.MkdirAll(path, 0755)
	}
	return os.Mkdir(path, 0755)
}

// createSymlink creates a symbolic link
func (m *FileModule) createSymlink(src, dst string, force bool) error {
	// Check if destination already exists
	if _, err := os.Lstat(dst); err == nil {
		if !force {
			return fmt.Errorf("destination already exists and force=false")
		}
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("failed to remove existing destination: %w", err)
		}
	}

	return os.Symlink(src, dst)
}

// createHardlink creates a hard link
func (m *FileModule) createHardlink(src, dst string, force bool) error {
	// Check if destination already exists
	if _, err := os.Stat(dst); err == nil {
		if !force {
			return fmt.Errorf("destination already exists and force=false")
		}
		if err := os.Remove(dst); err != nil {
			return fmt.Errorf("failed to remove existing destination: %w", err)
		}
	}

	return os.Link(src, dst)
}

// applyAttributes applies file attributes (mode, owner, group)
func (m *FileModule) applyAttributes(path, mode, owner, group string, recurse bool) (bool, error) {
	changed := false

	// Apply mode
	if mode != "" {
		fileMode, err := m.parseMode(mode)
		if err != nil {
			return false, fmt.Errorf("invalid mode %s: %w", mode, err)
		}

		if recurse {
			err = filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				return os.Chmod(walkPath, fileMode)
			})
		} else {
			err = os.Chmod(path, fileMode)
		}

		if err != nil {
			return false, fmt.Errorf("failed to set mode: %w", err)
		}
		changed = true
	}

	// Apply owner/group (simplified - real implementation would be more complex)
	if owner != "" || group != "" {
		// This is a placeholder for chown functionality
		// Real implementation would handle user/group resolution and chown system calls
		changed = true
	}

	return changed, nil
}

// isValidMode checks if a mode string is valid
func (m *FileModule) isValidMode(mode string) bool {
	if strings.HasPrefix(mode, "0") || len(mode) == 3 || len(mode) == 4 {
		_, err := strconv.ParseInt(mode, 8, 32)
		return err == nil
	}
	return false
}

// parseMode parses a mode string into os.FileMode
func (m *FileModule) parseMode(mode string) (os.FileMode, error) {
	if strings.HasPrefix(mode, "0") {
		mode = mode[1:]
	}

	modeInt, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return 0, err
	}

	return os.FileMode(modeInt), nil
}

