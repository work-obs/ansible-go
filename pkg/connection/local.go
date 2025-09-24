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

package connection

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// LocalConnection implements Connection for local execution
type LocalConnection struct {
	*BaseConnection
}

// NewLocalConnection creates a new local connection
func NewLocalConnection(config *ConnectionConfig) (Connection, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, err
	}

	// For local connections, override host to localhost
	config.Host = "localhost"

	return &LocalConnection{
		BaseConnection: NewBaseConnection(config),
	}, nil
}

// Connect establishes the connection (no-op for local)
func (c *LocalConnection) Connect(ctx context.Context) error {
	c.SetConnected(true)
	return nil
}

// Close closes the connection (no-op for local)
func (c *LocalConnection) Close() error {
	c.SetConnected(false)
	return nil
}

// GetConnectionType returns the connection type
func (c *LocalConnection) GetConnectionType() string {
	return "local"
}

// Execute runs a command on the local host
func (c *LocalConnection) Execute(ctx context.Context, cmd string, stdin io.Reader) (*ExecutionResult, error) {
	startTime := time.Now()

	// Apply become if configured
	if c.GetConfig().Become {
		cmd = c.ApplyBecome(cmd)
	}

	// Create command based on OS
	var execCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		execCmd = exec.CommandContext(ctx, "cmd", "/C", cmd)
	} else {
		execCmd = exec.CommandContext(ctx, "/bin/sh", "-c", cmd)
	}

	// Set up environment
	if env := c.GetConfig().Environment; env != nil {
		execCmd.Env = os.Environ()
		for key, value := range env {
			execCmd.Env = append(execCmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Set up stdin
	if stdin != nil {
		execCmd.Stdin = stdin
	}

	// Execute command and capture output
	stdout, stderr, exitCode, err := c.runCommand(execCmd)

	duration := time.Since(startTime)

	result := &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
		Error:    err,
	}

	return result, nil
}

// runCommand executes a command and returns stdout, stderr, exit code, and error
func (c *LocalConnection) runCommand(cmd *exec.Cmd) (string, string, int, error) {
	// Capture stdout and stderr separately
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return "", "", -1, fmt.Errorf("failed to start command: %w", err)
	}

	// Read stdout and stderr concurrently
	stdoutChan := make(chan string, 1)
	stderrChan := make(chan string, 1)

	go func() {
		stdout, _ := io.ReadAll(stdoutPipe)
		stdoutChan <- string(stdout)
	}()

	go func() {
		stderr, _ := io.ReadAll(stderrPipe)
		stderrChan <- string(stderr)
	}()

	// Wait for command to complete
	err = cmd.Wait()

	// Get output
	stdout := <-stdoutChan
	stderr := <-stderrChan

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			exitCode = exitError.ExitCode()
		} else {
			exitCode = -1
		}
	}

	return stdout, stderr, exitCode, err
}

// PutFile copies a file from local to local (essentially a copy operation)
func (c *LocalConnection) PutFile(ctx context.Context, localPath, remotePath string) error {
	// For local connection, this is just a file copy
	return c.copyFile(localPath, remotePath)
}

// GetFile copies a file from local to local (essentially a copy operation)
func (c *LocalConnection) GetFile(ctx context.Context, remotePath, localPath string) error {
	// For local connection, this is just a file copy
	return c.copyFile(remotePath, localPath)
}

// copyFile performs a local file copy
func (c *LocalConnection) copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	// Copy file permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	if err := dstFile.Chmod(srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// FileExists checks if a file exists on the local host
func (c *LocalConnection) FileExists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// CreateDirectory creates a directory on the local host
func (c *LocalConnection) CreateDirectory(ctx context.Context, path string, mode uint32) error {
	return os.MkdirAll(path, os.FileMode(mode))
}

// RemoveFile removes a file on the local host
func (c *LocalConnection) RemoveFile(ctx context.Context, path string) error {
	return os.Remove(path)
}

// GetFileInfo gets file information from the local host
func (c *LocalConnection) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	fileInfo := &FileInfo{
		Name:    stat.Name(),
		Size:    stat.Size(),
		Mode:    uint32(stat.Mode().Perm()),
		ModTime: stat.ModTime(),
		IsDir:   stat.IsDir(),
	}

	// Add owner/group information on Unix-like systems
	if runtime.GOOS != "windows" {
		if sys := stat.Sys(); sys != nil {
			// This would require syscall package and platform-specific code
			// For now, we'll leave owner/group empty
			fileInfo.Owner = ""
			fileInfo.Group = ""
		}
	}

	return fileInfo, nil
}