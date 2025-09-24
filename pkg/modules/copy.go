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
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/work-obs/ansible-go/pkg/plugins"
)

// CopyModule implements the copy module for copying files
type CopyModule struct {
	*BaseModule
}

// NewCopyModule creates a new copy module
func NewCopyModule() *CopyModule {
	return &CopyModule{
		BaseModule: NewBaseModule(
			"copy",
			"Copy files to remote locations",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Validate validates the module arguments
func (m *CopyModule) Validate(args map[string]interface{}) error {
	src := GetArgString(args, "src", "")
	content := GetArgString(args, "content", "")
	dest := GetArgString(args, "dest", "")

	// Must have destination
	if dest == "" {
		return fmt.Errorf("dest is required")
	}

	// Must have either src or content, but not both
	if src == "" && content == "" {
		return fmt.Errorf("either src or content is required")
	}
	if src != "" && content != "" {
		return fmt.Errorf("src and content are mutually exclusive")
	}

	// Validate backup
	backup := GetArgString(args, "backup", "")
	if backup != "" && backup != "yes" && backup != "no" {
		return fmt.Errorf("backup must be 'yes' or 'no'")
	}

	// Validate mode
	mode := GetArgString(args, "mode", "")
	if mode != "" {
		if !m.isValidMode(mode) {
			return fmt.Errorf("invalid mode: %s", mode)
		}
	}

	return nil
}

// Execute implements the ExecutablePlugin interface
func (m *CopyModule) Execute(ctx context.Context, moduleCtx *plugins.ModuleContext) (map[string]interface{}, error) {
	return RunModule(m, ctx, moduleCtx)
}

// Run executes the copy module
func (m *CopyModule) Run(ctx context.Context, args map[string]interface{}, config *config.Config) (*ModuleResult, error) {
	src := GetArgString(args, "src", "")
	content := GetArgString(args, "content", "")
	dest := GetArgString(args, "dest", "")
	backup := GetArgBool(args, "backup", false)
	mode := GetArgString(args, "mode", "")
	owner := GetArgString(args, "owner", "")
	group := GetArgString(args, "group", "")
	force := GetArgBool(args, "force", true)
	followLinks := GetArgBool(args, "follow", false)

	// Check if destination exists and is directory
	if destInfo, err := os.Stat(dest); err == nil && destInfo.IsDir() {
		if src != "" {
			// If dest is directory and we have src, append filename
			dest = filepath.Join(dest, filepath.Base(src))
		} else {
			return FailResult("dest is a directory but content was provided - specify full path", 1), nil
		}
	}

	// Check if we need to do anything
	changed := false
	needsCopy := true

	// Check if destination exists
	if _, err := os.Stat(dest); err == nil {
		if !force {
			return UnchangedResult("file already exists and force=no"), nil
		}

		// Compare checksums if source file exists
		if src != "" {
			srcChecksum, err := m.getFileChecksum(src)
			if err != nil {
				return FailResult(fmt.Sprintf("failed to get source checksum: %v", err), 1), nil
			}

			destChecksum, err := m.getFileChecksum(dest)
			if err != nil {
				return FailResult(fmt.Sprintf("failed to get dest checksum: %v", err), 1), nil
			}

			if srcChecksum == destChecksum {
				needsCopy = false
			}
		} else if content != "" {
			// Compare content with existing file
			existing, err := os.ReadFile(dest)
			if err != nil {
				return FailResult(fmt.Sprintf("failed to read existing file: %v", err), 1), nil
			}

			if string(existing) == content {
				needsCopy = false
			}
		}
	}

	// Check mode - don't actually copy in check mode
	if IsCheckMode(args) {
		if needsCopy {
			return ChangedResult("Would copy file"), nil
		}
		return UnchangedResult("File already exists with same content"), nil
	}

	// Create backup if requested
	var backupFile string
	if backup && needsCopy {
		if _, err := os.Stat(dest); err == nil {
			backupFile, err = m.createBackup(dest)
			if err != nil {
				return FailResult(fmt.Sprintf("failed to create backup: %v", err), 1), nil
			}
		}
	}

	// Copy the file
	if needsCopy {
		var copyErr error
		if src != "" {
			copyErr = m.copyFile(src, dest, followLinks)
		} else {
			copyErr = m.writeContent(content, dest)
		}

		if copyErr != nil {
			return FailResult(fmt.Sprintf("failed to copy: %v", copyErr), 1), nil
		}
		changed = true
	}

	// Set file permissions
	if mode != "" {
		fileMode, err := m.parseMode(mode)
		if err != nil {
			return FailResult(fmt.Sprintf("invalid mode %s: %v", mode, err), 1), nil
		}

		if err := os.Chmod(dest, fileMode); err != nil {
			return FailResult(fmt.Sprintf("failed to set mode: %v", err), 1), nil
		}
		changed = true
	}

	// Set ownership (simplified - would need more complex implementation for cross-platform)
	if owner != "" || group != "" {
		// This is a placeholder - real implementation would handle user/group changes
		// For now, we'll just note that it would change
		changed = true
	}

	// Prepare result
	result := &ModuleResult{
		Changed: changed,
		Results: make(map[string]interface{}),
	}

	// Add file information to results
	if destInfo, err := os.Stat(dest); err == nil {
		result.Results["dest"] = dest
		result.Results["size"] = destInfo.Size()
		result.Results["mode"] = fmt.Sprintf("%04o", destInfo.Mode().Perm())

		// Add checksums
		if checksum, err := m.getFileChecksum(dest); err == nil {
			result.Results["checksum"] = checksum
		}
		if md5sum, err := m.getFileMD5(dest); err == nil {
			result.Results["md5sum"] = md5sum
		}
	}

	if backupFile != "" {
		result.Results["backup_file"] = backupFile
	}

	if changed {
		result.Msg = "file copied"
	} else {
		result.Msg = "file already exists"
	}

	return result, nil
}

// copyFile copies a file from src to dest
func (m *CopyModule) copyFile(src, dest string, followLinks bool) error {
	// Open source file
	var srcFile *os.File
	var err error

	if followLinks {
		srcFile, err = os.Open(src)
	} else {
		// For symlinks, we'd need to handle them specially
		// For now, just open normally
		srcFile, err = os.Open(src)
	}

	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy file contents
	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Copy file permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	err = destFile.Chmod(srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// writeContent writes content to a file
func (m *CopyModule) writeContent(content, dest string) error {
	return os.WriteFile(dest, []byte(content), 0644)
}

// createBackup creates a backup of the file
func (m *CopyModule) createBackup(dest string) (string, error) {
	backupFile := dest + ".backup"
	return backupFile, m.copyFile(dest, backupFile, false)
}

// getFileChecksum gets SHA1 checksum of a file
func (m *CopyModule) getFileChecksum(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha1.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// getFileMD5 gets MD5 checksum of a file
func (m *CopyModule) getFileMD5(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := md5.New()
	_, err = io.Copy(hasher, file)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

// isValidMode checks if a mode string is valid
func (m *CopyModule) isValidMode(mode string) bool {
	// Check if it's a valid octal mode
	if strings.HasPrefix(mode, "0") || len(mode) == 3 || len(mode) == 4 {
		_, err := strconv.ParseInt(mode, 8, 32)
		return err == nil
	}

	// Could also support symbolic modes like "u+x,g-w" but that's more complex
	return false
}

// parseMode parses a mode string into os.FileMode
func (m *CopyModule) parseMode(mode string) (os.FileMode, error) {
	// Parse octal mode
	if strings.HasPrefix(mode, "0") {
		mode = mode[1:] // Remove leading 0
	}

	modeInt, err := strconv.ParseInt(mode, 8, 32)
	if err != nil {
		return 0, err
	}

	return os.FileMode(modeInt), nil
}

