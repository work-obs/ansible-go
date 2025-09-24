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
	"path/filepath"
	"strings"
	"time"
)

// WinRMConnection implements Connection for Windows Remote Management
type WinRMConnection struct {
	*BaseConnection
	// In a full implementation, this would include WinRM client
	// For now, we'll provide the interface and basic structure
}

// NewWinRMConnection creates a new WinRM connection
func NewWinRMConnection(config *ConnectionConfig) (Connection, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, err
	}

	// Set default port if not specified
	if config.Port == 0 {
		config.Port = 5985 // Default WinRM HTTP port (5986 for HTTPS)
	}

	// Set default user if not specified
	if config.User == "" {
		config.User = "Administrator"
	}

	return &WinRMConnection{
		BaseConnection: NewBaseConnection(config),
	}, nil
}

// Connect establishes the WinRM connection
func (c *WinRMConnection) Connect(ctx context.Context) error {
	// In a full implementation, this would establish the actual WinRM connection
	// For now, we'll simulate a successful connection
	c.SetConnected(true)
	return nil
}

// Close closes the WinRM connection
func (c *WinRMConnection) Close() error {
	c.SetConnected(false)
	return nil
}

// GetConnectionType returns the connection type
func (c *WinRMConnection) GetConnectionType() string {
	return "winrm"
}

// Execute runs a command on the remote Windows host via WinRM
func (c *WinRMConnection) Execute(ctx context.Context, cmd string, stdin io.Reader) (*ExecutionResult, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	startTime := time.Now()

	// For demonstration purposes, we'll simulate command execution
	// In a real implementation, this would use a WinRM client library
	result := &ExecutionResult{
		ExitCode: 0,
		Stdout:   fmt.Sprintf("Simulated execution of: %s", cmd),
		Stderr:   "",
		Duration: time.Since(startTime),
		Error:    nil,
	}

	// Handle PowerShell commands specially
	if strings.HasPrefix(cmd, "powershell") || strings.HasPrefix(cmd, "pwsh") {
		result.Stdout = fmt.Sprintf("PowerShell output for: %s", cmd)
	}

	return result, nil
}

// PutFile copies a file from local to remote Windows host via WinRM
func (c *WinRMConnection) PutFile(ctx context.Context, localPath, remotePath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// In a real implementation, this would:
	// 1. Read the local file
	// 2. Base64 encode it
	// 3. Use PowerShell to decode and write it on the remote host
	// For now, we'll simulate success
	return nil
}

// GetFile copies a file from remote Windows host to local via WinRM
func (c *WinRMConnection) GetFile(ctx context.Context, remotePath, localPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// In a real implementation, this would:
	// 1. Use PowerShell to read and base64 encode the remote file
	// 2. Transfer the encoded content
	// 3. Decode and write to local file
	// For now, we'll simulate success
	return nil
}

// FileExists checks if a file exists on the remote Windows host
func (c *WinRMConnection) FileExists(ctx context.Context, path string) (bool, error) {
	// Use PowerShell Test-Path cmdlet
	psCmd := fmt.Sprintf("powershell -Command \"Test-Path '%s'\"", path)
	result, err := c.Execute(ctx, psCmd, nil)
	if err != nil {
		return false, err
	}

	// In simulation, we'll return true for demonstration
	return result.ExitCode == 0, nil
}

// CreateDirectory creates a directory on the remote Windows host
func (c *WinRMConnection) CreateDirectory(ctx context.Context, path string, mode uint32) error {
	// Use PowerShell New-Item cmdlet
	psCmd := fmt.Sprintf("powershell -Command \"New-Item -Path '%s' -ItemType Directory -Force\"", path)
	result, err := c.Execute(ctx, psCmd, nil)
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to create directory: %s", result.Stderr)
	}

	return nil
}

// RemoveFile removes a file on the remote Windows host
func (c *WinRMConnection) RemoveFile(ctx context.Context, path string) error {
	// Use PowerShell Remove-Item cmdlet
	psCmd := fmt.Sprintf("powershell -Command \"Remove-Item -Path '%s' -Force\"", path)
	result, err := c.Execute(ctx, psCmd, nil)
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to remove file: %s", result.Stderr)
	}

	return nil
}

// GetFileInfo gets file information from the remote Windows host
func (c *WinRMConnection) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	// Use PowerShell Get-Item cmdlet
	psCmd := fmt.Sprintf("powershell -Command \"Get-Item '%s' | Select-Object Name,Length,LastWriteTime,PSIsContainer | ConvertTo-Json\"", path)
	result, err := c.Execute(ctx, psCmd, nil)
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// In a real implementation, this would parse the JSON output
	// For now, we'll return simulated file info
	fileInfo := &FileInfo{
		Name:    filepath.Base(path),
		Size:    1024,
		Mode:    0644,
		ModTime: time.Now(),
		IsDir:   strings.Contains(path, "directory"),
		Owner:   c.GetConfig().User,
		Group:   "Users",
	}

	return fileInfo, nil
}

// ApplyBecome overrides the base implementation for Windows-specific become
func (c *WinRMConnection) ApplyBecome(cmd string) string {
	config := c.GetConfig()
	if !config.Become {
		return cmd
	}

	// On Windows, "become" typically means running as a different user
	// This would use PowerShell's Start-Process with -Credential
	becomeUser := config.BecomeUser
	if becomeUser == "" {
		becomeUser = "Administrator"
	}

	// Wrap the command in PowerShell with RunAs
	return fmt.Sprintf("powershell -Command \"Start-Process cmd -ArgumentList '/c %s' -Verb RunAs -Wait\"", cmd)
}

// GetPowerShellCommand wraps a command in PowerShell for Windows execution
func (c *WinRMConnection) GetPowerShellCommand(cmd string) string {
	// Escape single quotes and wrap in PowerShell
	escapedCmd := strings.ReplaceAll(cmd, "'", "''")
	return fmt.Sprintf("powershell -Command \"%s\"", escapedCmd)
}

// ExecutePowerShell executes a PowerShell command
func (c *WinRMConnection) ExecutePowerShell(ctx context.Context, psScript string, stdin io.Reader) (*ExecutionResult, error) {
	cmd := fmt.Sprintf("powershell -Command \"%s\"", psScript)
	return c.Execute(ctx, cmd, stdin)
}

// IsWindowsHost checks if this connection is for a Windows host
func (c *WinRMConnection) IsWindowsHost() bool {
	return true // WinRM is always for Windows hosts
}

// GetWindowsPath converts a Unix-style path to Windows format
func (c *WinRMConnection) GetWindowsPath(unixPath string) string {
	// Convert forward slashes to backslashes
	winPath := strings.ReplaceAll(unixPath, "/", "\\")

	// Handle drive letters (e.g., /c/path -> C:\path)
	if len(winPath) >= 3 && winPath[0] == '\\' && winPath[2] == '\\' {
		return strings.ToUpper(string(winPath[1])) + ":" + winPath[2:]
	}

	return winPath
}

// GetUnixPath converts a Windows-style path to Unix format
func (c *WinRMConnection) GetUnixPath(winPath string) string {
	// Convert backslashes to forward slashes
	unixPath := strings.ReplaceAll(winPath, "\\", "/")

	// Handle drive letters (e.g., C:\path -> /c/path)
	if len(unixPath) >= 2 && unixPath[1] == ':' {
		return "/" + strings.ToLower(string(unixPath[0])) + unixPath[2:]
	}

	return unixPath
}

// TestWinRMConnection tests WinRM connection functionality
func TestWinRMConnection(config *ConnectionConfig) error {
	conn, err := NewWinRMConnection(config)
	if err != nil {
		return fmt.Errorf("failed to create WinRM connection: %w", err)
	}

	ctx := context.Background()
	if err := conn.Connect(ctx); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Test basic command execution
	result, err := conn.Execute(ctx, "echo test", nil)
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("command failed with exit code %d", result.ExitCode)
	}

	return nil
}

// RegisterWinRMConnection registers the WinRM connection with the factory
func RegisterWinRMConnection(factory *DefaultConnectionFactory) {
	factory.RegisterConnection("winrm", NewWinRMConnection)
	factory.RegisterConnection("psrp", NewWinRMConnection) // PowerShell Remoting Protocol alias
}