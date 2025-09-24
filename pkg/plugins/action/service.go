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
	"os/exec"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// ServiceActionPlugin implements the service action plugin
type ServiceActionPlugin struct {
	*BaseActionPlugin
}

// NewServiceActionPlugin creates a new service action plugin
func NewServiceActionPlugin() *ServiceActionPlugin {
	return &ServiceActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"service",
			"Manage services",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Run executes the service action plugin
func (a *ServiceActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	name := GetArgString(args, "name", "")
	state := GetArgString(args, "state", "")

	// Validate arguments
	if name == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "name is required",
		}, nil
	}

	if state != "" {
		validStates := []string{"started", "stopped", "restarted", "reloaded"}
		if err := ValidateChoices(args, "state", validStates); err != nil {
			return &plugins.ActionResult{
				Failed:  true,
				Message: err.Error(),
			}, nil
		}
	}

	// Check mode handling
	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: true,
			Message: fmt.Sprintf("Would manage service %s", name),
		}, nil
	}

	// Detect service manager and get current status
	serviceManager := a.detectServiceManager()
	currentRunning := a.isServiceRunning(name, serviceManager)

	changed := false
	var err error

	// Handle state changes
	if state != "" {
		switch state {
		case "started":
			if !currentRunning {
				err = a.startService(name, serviceManager)
				changed = true
			}
		case "stopped":
			if currentRunning {
				err = a.stopService(name, serviceManager)
				changed = true
			}
		case "restarted":
			err = a.restartService(name, serviceManager)
			changed = true
		case "reloaded":
			err = a.reloadService(name, serviceManager)
			changed = true
		}
	}

	if err != nil {
		return &plugins.ActionResult{
			Failed:  true,
			Message: fmt.Sprintf("Service operation failed: %v", err),
		}, nil
	}

	result := &plugins.ActionResult{
		Changed: changed,
		Results: make(map[string]interface{}),
	}

	result.Results["name"] = name
	if state != "" {
		result.Results["state"] = state
	}

	if changed {
		result.Message = fmt.Sprintf("Service %s %s", name, state)
	} else {
		result.Message = fmt.Sprintf("Service %s already in desired state", name)
	}

	return result, nil
}

// detectServiceManager detects which service manager is available
func (a *ServiceActionPlugin) detectServiceManager() string {
	if a.commandExists("systemctl") {
		return "systemd"
	}
	if a.commandExists("service") {
		return "service"
	}
	return "unknown"
}

// commandExists checks if a command exists
func (a *ServiceActionPlugin) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// isServiceRunning checks if a service is currently running
func (a *ServiceActionPlugin) isServiceRunning(name, manager string) bool {
	switch manager {
	case "systemd":
		cmd := exec.Command("systemctl", "is-active", "--quiet", name)
		return cmd.Run() == nil
	case "service":
		cmd := exec.Command("service", name, "status")
		return cmd.Run() == nil
	default:
		return false
	}
}

// startService starts a service
func (a *ServiceActionPlugin) startService(name, manager string) error {
	switch manager {
	case "systemd":
		return exec.Command("systemctl", "start", name).Run()
	case "service":
		return exec.Command("service", name, "start").Run()
	default:
		return fmt.Errorf("unsupported service manager: %s", manager)
	}
}

// stopService stops a service
func (a *ServiceActionPlugin) stopService(name, manager string) error {
	switch manager {
	case "systemd":
		return exec.Command("systemctl", "stop", name).Run()
	case "service":
		return exec.Command("service", name, "stop").Run()
	default:
		return fmt.Errorf("unsupported service manager: %s", manager)
	}
}

// restartService restarts a service
func (a *ServiceActionPlugin) restartService(name, manager string) error {
	switch manager {
	case "systemd":
		return exec.Command("systemctl", "restart", name).Run()
	case "service":
		return exec.Command("service", name, "restart").Run()
	default:
		return fmt.Errorf("unsupported service manager: %s", manager)
	}
}

// reloadService reloads a service
func (a *ServiceActionPlugin) reloadService(name, manager string) error {
	switch manager {
	case "systemd":
		return exec.Command("systemctl", "reload", name).Run()
	case "service":
		return exec.Command("service", name, "reload").Run()
	default:
		return fmt.Errorf("unsupported service manager: %s", manager)
	}
}