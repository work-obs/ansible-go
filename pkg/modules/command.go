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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/work-obs/ansible-go/pkg/plugins"
)

// CommandModule implements the command module for executing shell commands
type CommandModule struct {
	*BaseModule
}

// NewCommandModule creates a new command module
func NewCommandModule() *CommandModule {
	return &CommandModule{
		BaseModule: NewBaseModule(
			"command",
			"Execute commands on targets",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Validate validates the module arguments
func (m *CommandModule) Validate(args map[string]interface{}) error {
	// Check if we have either a command or _raw_params
	cmd := GetArgString(args, "cmd", "")
	rawParams := GetArgString(args, "_raw_params", "")

	if cmd == "" && rawParams == "" {
		return fmt.Errorf("command module requires either 'cmd' or command as argument")
	}

	// Validate creates argument
	if creates := GetArgString(args, "creates", ""); creates != "" {
		if !filepath.IsAbs(creates) {
			return fmt.Errorf("creates path must be absolute")
		}
	}

	// Validate removes argument
	if removes := GetArgString(args, "removes", ""); removes != "" {
		if !filepath.IsAbs(removes) {
			return fmt.Errorf("removes path must be absolute")
		}
	}

	// Validate chdir argument
	if chdir := GetArgString(args, "chdir", ""); chdir != "" {
		if !filepath.IsAbs(chdir) {
			return fmt.Errorf("chdir path must be absolute")
		}
	}

	return nil
}

// Execute implements the ExecutablePlugin interface
func (m *CommandModule) Execute(ctx context.Context, moduleCtx *plugins.ModuleContext) (map[string]interface{}, error) {
	return RunModule(m, ctx, moduleCtx)
}

// Run executes the command module
func (m *CommandModule) Run(ctx context.Context, args map[string]interface{}, config *config.Config) (*ModuleResult, error) {
	// Get the command to execute
	cmd := GetArgString(args, "cmd", "")
	if cmd == "" {
		cmd = GetArgString(args, "_raw_params", "")
	}

	if cmd == "" {
		return FailResult("No command specified", 1), nil
	}

	// Check for unsafe characters if warn is enabled (default)
	warn := GetArgBool(args, "warn", true)
	if warn && m.hasUnsafeShellChars(cmd) {
		return FailResult(fmt.Sprintf("Command contains potentially unsafe shell characters. Use shell module instead: %s", cmd), 1), nil
	}

	// Get other arguments
	chdir := GetArgString(args, "chdir", "")
	creates := GetArgString(args, "creates", "")
	removes := GetArgString(args, "removes", "")
	timeout := GetArgInt(args, "timeout", 30)

	// Check creates condition
	if creates != "" {
		if _, err := os.Stat(creates); err == nil {
			return UnchangedResult(fmt.Sprintf("skipped, since %s exists", creates)), nil
		}
	}

	// Check removes condition
	if removes != "" {
		if _, err := os.Stat(removes); os.IsNotExist(err) {
			return UnchangedResult(fmt.Sprintf("skipped, since %s does not exist", removes)), nil
		}
	}

	// Check mode - don't execute in check mode
	if IsCheckMode(args) {
		return ChangedResult("Would execute command"), nil
	}

	// Execute the command
	result, err := m.executeCommand(ctx, cmd, chdir, time.Duration(timeout)*time.Second)
	if err != nil {
		return FailResult(fmt.Sprintf("Failed to execute command: %v", err), 1), nil
	}

	return result, nil
}

// executeCommand executes a shell command with the given parameters
func (m *CommandModule) executeCommand(ctx context.Context, cmdStr, chdir string, timeout time.Duration) (*ModuleResult, error) {
	// Split command into executable and arguments
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return FailResult("Empty command", 1), nil
	}

	executable := parts[0]
	args := parts[1:]

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(cmdCtx, executable, args...)

	// Set working directory if specified
	if chdir != "" {
		cmd.Dir = chdir
	}

	// Execute command and capture output
	stdout, stderr, exitCode, err := m.runCommandWithOutput(cmd)

	result := &ModuleResult{
		Changed: true, // Command module is always considered to make changes
		Stdout:  stdout,
		Stderr:  stderr,
		RC:      exitCode,
	}

	// Handle different types of errors
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Command executed but returned non-zero exit code
			result.Failed = true
			result.Msg = "non-zero return code"
			if stderr != "" {
				result.Msg = fmt.Sprintf("non-zero return code: %s", stderr)
			}
		} else if err == context.DeadlineExceeded {
			// Command timed out
			result.Failed = true
			result.Msg = fmt.Sprintf("command timed out after %v", timeout)
		} else {
			// Other execution error
			result.Failed = true
			result.Msg = fmt.Sprintf("failed to execute command: %v", err)
		}
	} else if exitCode != 0 {
		// Command completed but with non-zero exit code
		result.Failed = true
		result.Msg = "non-zero return code"
	}

	// Add command info to results
	if result.Results == nil {
		result.Results = make(map[string]interface{})
	}
	result.Results["cmd"] = cmdStr
	result.Results["delta"] = "0:00:00.123456" // Simplified - real implementation would measure time
	result.Results["start"] = time.Now().Format("2006-01-02 15:04:05.000000")
	result.Results["end"] = time.Now().Format("2006-01-02 15:04:05.000000")

	return result, nil
}

// runCommandWithOutput runs a command and returns stdout, stderr, exit code, and error
func (m *CommandModule) runCommandWithOutput(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	// Capture stdout and stderr
	stdoutBytes, err := cmd.Output()
	if err != nil {
		// Check if it's an ExitError to get stderr
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else {
		exitCode = 0
	}

	stdout = string(stdoutBytes)
	return
}

// hasUnsafeShellChars checks if the command contains potentially unsafe shell characters
func (m *CommandModule) hasUnsafeShellChars(cmd string) bool {
	unsafeChars := []string{"|", ";", "&", "$", "`", "\\", "\"", "'", "<", ">", "(", ")", "{", "}", "[", "]", "~", "*", "?"}

	for _, char := range unsafeChars {
		if strings.Contains(cmd, char) {
			return true
		}
	}
	return false
}

// ShellModule implements the shell module (similar to command but allows shell features)
type ShellModule struct {
	*BaseModule
}

// NewShellModule creates a new shell module
func NewShellModule() *ShellModule {
	return &ShellModule{
		BaseModule: NewBaseModule(
			"shell",
			"Execute shell commands on targets",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Validate validates the shell module arguments
func (m *ShellModule) Validate(args map[string]interface{}) error {
	// Similar validation to command module
	cmd := GetArgString(args, "cmd", "")
	rawParams := GetArgString(args, "_raw_params", "")

	if cmd == "" && rawParams == "" {
		return fmt.Errorf("shell module requires either 'cmd' or command as argument")
	}

	return nil
}

// Execute implements the ExecutablePlugin interface
func (m *ShellModule) Execute(ctx context.Context, moduleCtx *plugins.ModuleContext) (map[string]interface{}, error) {
	return RunModule(m, ctx, moduleCtx)
}

// Run executes the shell module
func (m *ShellModule) Run(ctx context.Context, args map[string]interface{}, config *config.Config) (*ModuleResult, error) {
	// Get the command to execute
	cmd := GetArgString(args, "cmd", "")
	if cmd == "" {
		cmd = GetArgString(args, "_raw_params", "")
	}

	if cmd == "" {
		return FailResult("No command specified", 1), nil
	}

	// Get other arguments
	chdir := GetArgString(args, "chdir", "")
	creates := GetArgString(args, "creates", "")
	removes := GetArgString(args, "removes", "")
	timeout := GetArgInt(args, "timeout", 30)

	// Check creates condition
	if creates != "" {
		if _, err := os.Stat(creates); err == nil {
			return UnchangedResult(fmt.Sprintf("skipped, since %s exists", creates)), nil
		}
	}

	// Check removes condition
	if removes != "" {
		if _, err := os.Stat(removes); os.IsNotExist(err) {
			return UnchangedResult(fmt.Sprintf("skipped, since %s does not exist", removes)), nil
		}
	}

	// Check mode - don't execute in check mode
	if IsCheckMode(args) {
		return ChangedResult("Would execute shell command"), nil
	}

	// Execute the command using shell
	result, err := m.executeShellCommand(ctx, cmd, chdir, time.Duration(timeout)*time.Second)
	if err != nil {
		return FailResult(fmt.Sprintf("Failed to execute shell command: %v", err), 1), nil
	}

	return result, nil
}

// executeShellCommand executes a command through the shell
func (m *ShellModule) executeShellCommand(ctx context.Context, cmdStr, chdir string, timeout time.Duration) (*ModuleResult, error) {
	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Use appropriate shell based on OS
	var cmd *exec.Cmd
	if os.PathSeparator == '\\' {
		// Windows
		cmd = exec.CommandContext(cmdCtx, "cmd", "/C", cmdStr)
	} else {
		// Unix-like
		cmd = exec.CommandContext(cmdCtx, "/bin/sh", "-c", cmdStr)
	}

	// Set working directory if specified
	if chdir != "" {
		cmd.Dir = chdir
	}

	// Execute command and capture output
	stdout, stderr, exitCode, err := m.runShellCommandWithOutput(cmd)

	result := &ModuleResult{
		Changed: true, // Shell module is always considered to make changes
		Stdout:  stdout,
		Stderr:  stderr,
		RC:      exitCode,
	}

	// Handle errors similar to command module
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			result.Failed = true
			result.Msg = "non-zero return code"
			if stderr != "" {
				result.Msg = fmt.Sprintf("non-zero return code: %s", stderr)
			}
		} else if err == context.DeadlineExceeded {
			result.Failed = true
			result.Msg = fmt.Sprintf("command timed out after %v", timeout)
		} else {
			result.Failed = true
			result.Msg = fmt.Sprintf("failed to execute shell command: %v", err)
		}
	} else if exitCode != 0 {
		result.Failed = true
		result.Msg = "non-zero return code"
	}

	// Add command info to results
	if result.Results == nil {
		result.Results = make(map[string]interface{})
	}
	result.Results["cmd"] = cmdStr
	result.Results["delta"] = "0:00:00.123456"
	result.Results["start"] = time.Now().Format("2006-01-02 15:04:05.000000")
	result.Results["end"] = time.Now().Format("2006-01-02 15:04:05.000000")

	return result, nil
}

// runShellCommandWithOutput runs a shell command and returns stdout, stderr, exit code, and error
func (m *ShellModule) runShellCommandWithOutput(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	// Similar to command module but expects shell execution
	stdoutBytes, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr = string(exitErr.Stderr)
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	} else {
		exitCode = 0
	}

	stdout = string(stdoutBytes)
	return
}

