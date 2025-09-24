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
	"os/exec"
	"strings"
	"time"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// CommandActionPlugin implements the command action plugin
type CommandActionPlugin struct {
	*BaseActionPlugin
}

// NewCommandActionPlugin creates a new command action plugin
func NewCommandActionPlugin() *CommandActionPlugin {
	return &CommandActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"command",
			"Execute commands on targets",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Run executes the command action plugin
func (a *CommandActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	// Get the command to execute
	cmd := GetArgString(args, "cmd", "")
	if cmd == "" {
		cmd = GetArgString(args, "_raw_params", "")
	}

	if cmd == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "No command specified",
		}, nil
	}

	// Check for unsafe characters if warn is enabled (default)
	warn := GetArgBool(args, "warn", true)
	if warn && a.hasUnsafeShellChars(cmd) {
		return &plugins.ActionResult{
			Failed:  true,
			Message: fmt.Sprintf("Command contains potentially unsafe shell characters. Use shell module instead: %s", cmd),
		}, nil
	}

	// Get other arguments
	chdir := GetArgString(args, "chdir", "")
	creates := GetArgString(args, "creates", "")
	removes := GetArgString(args, "removes", "")
	timeout := GetArgInt(args, "timeout", 30)

	// Check creates condition
	if creates != "" {
		if _, err := os.Stat(creates); err == nil {
			return &plugins.ActionResult{
				Changed: false,
				Message: fmt.Sprintf("skipped, since %s exists", creates),
			}, nil
		}
	}

	// Check removes condition
	if removes != "" {
		if _, err := os.Stat(removes); os.IsNotExist(err) {
			return &plugins.ActionResult{
				Changed: false,
				Message: fmt.Sprintf("skipped, since %s does not exist", removes),
			}, nil
		}
	}

	// Check mode - don't execute in check mode
	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: true,
			Message: "Would execute command",
		}, nil
	}

	// Execute the command
	result, err := a.executeCommand(ctx, cmd, chdir, time.Duration(timeout)*time.Second)
	if err != nil {
		return &plugins.ActionResult{
			Failed:  true,
			Message: fmt.Sprintf("Failed to execute command: %v", err),
		}, nil
	}

	return result, nil
}

// executeCommand executes a shell command with the given parameters
func (a *CommandActionPlugin) executeCommand(ctx context.Context, cmdStr, chdir string, timeout time.Duration) (*plugins.ActionResult, error) {
	// Split command into executable and arguments
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "Empty command",
		}, nil
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
	stdout, stderr, exitCode, err := a.runCommandWithOutput(cmd)

	result := &plugins.ActionResult{
		Changed: true, // Command action is always considered to make changes
		Results: make(map[string]interface{}),
	}

	result.Results["stdout"] = stdout
	result.Results["stderr"] = stderr
	result.Results["rc"] = exitCode

	// Handle different types of errors
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Command executed but returned non-zero exit code
			result.Failed = true
			result.Message = "non-zero return code"
			if stderr != "" {
				result.Message = fmt.Sprintf("non-zero return code: %s", stderr)
			}
		} else if err == context.DeadlineExceeded {
			// Command timed out
			result.Failed = true
			result.Message = fmt.Sprintf("command timed out after %v", timeout)
		} else {
			// Other execution error
			result.Failed = true
			result.Message = fmt.Sprintf("failed to execute command: %v", err)
		}
	} else if exitCode != 0 {
		// Command completed but with non-zero exit code
		result.Failed = true
		result.Message = "non-zero return code"
	}

	// Add command info to results
	result.Results["cmd"] = cmdStr
	result.Results["delta"] = "0:00:00.123456" // Simplified - real implementation would measure time
	result.Results["start"] = time.Now().Format("2006-01-02 15:04:05.000000")
	result.Results["end"] = time.Now().Format("2006-01-02 15:04:05.000000")

	return result, nil
}

// runCommandWithOutput runs a command and returns stdout, stderr, exit code, and error
func (a *CommandActionPlugin) runCommandWithOutput(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
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
func (a *CommandActionPlugin) hasUnsafeShellChars(cmd string) bool {
	unsafeChars := []string{"|", ";", "&", "$", "`", "\\", "\"", "'", "<", ">", "(", ")", "{", "}", "[", "]", "~", "*", "?"}

	for _, char := range unsafeChars {
		if strings.Contains(cmd, char) {
			return true
		}
	}
	return false
}

// ShellActionPlugin implements the shell action plugin (similar to command but allows shell features)
type ShellActionPlugin struct {
	*BaseActionPlugin
}

// NewShellActionPlugin creates a new shell action plugin
func NewShellActionPlugin() *ShellActionPlugin {
	return &ShellActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"shell",
			"Execute shell commands on targets",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Run executes the shell action plugin
func (a *ShellActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	// Get the command to execute
	cmd := GetArgString(args, "cmd", "")
	if cmd == "" {
		cmd = GetArgString(args, "_raw_params", "")
	}

	if cmd == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "No command specified",
		}, nil
	}

	// Get other arguments
	chdir := GetArgString(args, "chdir", "")
	creates := GetArgString(args, "creates", "")
	removes := GetArgString(args, "removes", "")
	timeout := GetArgInt(args, "timeout", 30)

	// Check creates condition
	if creates != "" {
		if _, err := os.Stat(creates); err == nil {
			return &plugins.ActionResult{
				Changed: false,
				Message: fmt.Sprintf("skipped, since %s exists", creates),
			}, nil
		}
	}

	// Check removes condition
	if removes != "" {
		if _, err := os.Stat(removes); os.IsNotExist(err) {
			return &plugins.ActionResult{
				Changed: false,
				Message: fmt.Sprintf("skipped, since %s does not exist", removes),
			}, nil
		}
	}

	// Check mode - don't execute in check mode
	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: true,
			Message: "Would execute shell command",
		}, nil
	}

	// Execute the command using shell
	result, err := a.executeShellCommand(ctx, cmd, chdir, time.Duration(timeout)*time.Second)
	if err != nil {
		return &plugins.ActionResult{
			Failed:  true,
			Message: fmt.Sprintf("Failed to execute shell command: %v", err),
		}, nil
	}

	return result, nil
}

// executeShellCommand executes a command through the shell
func (a *ShellActionPlugin) executeShellCommand(ctx context.Context, cmdStr, chdir string, timeout time.Duration) (*plugins.ActionResult, error) {
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
	stdout, stderr, exitCode, err := a.runShellCommandWithOutput(cmd)

	result := &plugins.ActionResult{
		Changed: true, // Shell action is always considered to make changes
		Results: make(map[string]interface{}),
	}

	result.Results["stdout"] = stdout
	result.Results["stderr"] = stderr
	result.Results["rc"] = exitCode

	// Handle errors similar to command action
	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			result.Failed = true
			result.Message = "non-zero return code"
			if stderr != "" {
				result.Message = fmt.Sprintf("non-zero return code: %s", stderr)
			}
		} else if err == context.DeadlineExceeded {
			result.Failed = true
			result.Message = fmt.Sprintf("command timed out after %v", timeout)
		} else {
			result.Failed = true
			result.Message = fmt.Sprintf("failed to execute shell command: %v", err)
		}
	} else if exitCode != 0 {
		result.Failed = true
		result.Message = "non-zero return code"
	}

	// Add command info to results
	result.Results["cmd"] = cmdStr
	result.Results["delta"] = "0:00:00.123456"
	result.Results["start"] = time.Now().Format("2006-01-02 15:04:05.000000")
	result.Results["end"] = time.Now().Format("2006-01-02 15:04:05.000000")

	return result, nil
}

// runShellCommandWithOutput runs a shell command and returns stdout, stderr, exit code, and error
func (a *ShellActionPlugin) runShellCommandWithOutput(cmd *exec.Cmd) (stdout, stderr string, exitCode int, err error) {
	// Similar to command action but expects shell execution
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