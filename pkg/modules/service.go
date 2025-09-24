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
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/work-obs/ansible-go/pkg/plugins"
)

// ServiceModule implements the service module for managing system services
type ServiceModule struct {
	*BaseModule
}

// NewServiceModule creates a new service module
func NewServiceModule() *ServiceModule {
	return &ServiceModule{
		BaseModule: NewBaseModule(
			"service",
			"Manage services",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Validate validates the module arguments
func (m *ServiceModule) Validate(args map[string]interface{}) error {
	name := GetArgString(args, "name", "")
	if name == "" {
		return fmt.Errorf("name is required")
	}

	// Validate state
	state := GetArgString(args, "state", "")
	if state != "" {
		validStates := []string{"started", "stopped", "restarted", "reloaded"}
		if err := ValidateChoices(args, "state", validStates); err != nil {
			return err
		}
	}

	return nil
}

// Execute implements the ExecutablePlugin interface
func (m *ServiceModule) Execute(ctx context.Context, moduleCtx *plugins.ModuleContext) (map[string]interface{}, error) {
	return RunModule(m, ctx, moduleCtx)
}

// Run executes the service module
func (m *ServiceModule) Run(ctx context.Context, args map[string]interface{}, config *config.Config) (*ModuleResult, error) {
	name := GetArgString(args, "name", "")
	state := GetArgString(args, "state", "")
	enabled := args["enabled"] // Can be bool, string, or nil
	sleep := GetArgInt(args, "sleep", 1)

	// Detect service manager
	serviceManager, err := m.detectServiceManager()
	if err != nil {
		return FailResult(fmt.Sprintf("failed to detect service manager: %v", err), 1), nil
	}

	// Get current service status
	currentStatus, err := m.getServiceStatus(serviceManager, name)
	if err != nil {
		return FailResult(fmt.Sprintf("failed to get service status: %v", err), 1), nil
	}

	changed := false
	actions := []string{}

	// Check mode - don't actually make changes
	if IsCheckMode(args) {
		return m.handleCheckMode(name, state, enabled, currentStatus)
	}

	// Handle state changes
	if state != "" {
		stateChanged, action, err := m.handleState(serviceManager, name, state, currentStatus, sleep)
		if err != nil {
			return FailResult(fmt.Sprintf("failed to change service state: %v", err), 1), nil
		}
		if stateChanged {
			changed = true
			actions = append(actions, action)
		}
	}

	// Handle enabled changes
	if enabled != nil {
		enabledChanged, action, err := m.handleEnabled(serviceManager, name, enabled, currentStatus)
		if err != nil {
			return FailResult(fmt.Sprintf("failed to change service enabled state: %v", err), 1), nil
		}
		if enabledChanged {
			changed = true
			actions = append(actions, action)
		}
	}

	// Get final status
	finalStatus, err := m.getServiceStatus(serviceManager, name)
	if err != nil {
		// Don't fail here, just use current status
		finalStatus = currentStatus
	}

	// Prepare result
	result := &ModuleResult{
		Changed: changed,
		Results: make(map[string]interface{}),
	}

	result.Results["name"] = name
	result.Results["state"] = m.mapStatusToState(finalStatus.Running)
	result.Results["status"] = finalStatus.Status
	result.Results["enabled"] = finalStatus.Enabled

	if len(actions) > 0 {
		result.Msg = fmt.Sprintf("Service %s: %s", name, strings.Join(actions, ", "))
	} else {
		result.Msg = fmt.Sprintf("Service %s is already in desired state", name)
	}

	return result, nil
}

// ServiceManager represents the type of service manager
type ServiceManager string

const (
	ServiceManagerSystemd ServiceManager = "systemd"
	ServiceManagerService ServiceManager = "service"
	ServiceManagerSysV    ServiceManager = "sysv"
	ServiceManagerUnknown ServiceManager = "unknown"
)

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	Name     string
	Running  bool
	Enabled  bool
	Status   string
	ExitCode int
}

// detectServiceManager detects which service manager is available
func (m *ServiceModule) detectServiceManager() (ServiceManager, error) {
	// Check for systemd
	if m.commandExists("systemctl") {
		return ServiceManagerSystemd, nil
	}

	// Check for service command
	if m.commandExists("service") {
		return ServiceManagerService, nil
	}

	// Check for traditional SysV init scripts
	if runtime.GOOS == "linux" {
		return ServiceManagerSysV, nil
	}

	return ServiceManagerUnknown, fmt.Errorf("no supported service manager found")
}

// commandExists checks if a command exists in PATH
func (m *ServiceModule) commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// getServiceStatus gets the current status of a service
func (m *ServiceModule) getServiceStatus(manager ServiceManager, name string) (*ServiceStatus, error) {
	status := &ServiceStatus{
		Name:    name,
		Running: false,
		Enabled: false,
		Status:  "unknown",
	}

	switch manager {
	case ServiceManagerSystemd:
		return m.getSystemdStatus(name)
	case ServiceManagerService:
		return m.getServiceStatus_Service(name)
	case ServiceManagerSysV:
		return m.getSysVStatus(name)
	default:
		return status, fmt.Errorf("unsupported service manager")
	}
}

// getSystemdStatus gets service status using systemctl
func (m *ServiceModule) getSystemdStatus(name string) (*ServiceStatus, error) {
	status := &ServiceStatus{Name: name}

	// Check if service is running
	cmd := exec.Command("systemctl", "is-active", name)
	output, _ := cmd.Output()
	activeStatus := strings.TrimSpace(string(output))

	status.Running = activeStatus == "active"
	status.Status = activeStatus

	// Check if service is enabled
	cmd = exec.Command("systemctl", "is-enabled", name)
	output, _ = cmd.Output()
	enabledStatus := strings.TrimSpace(string(output))

	status.Enabled = enabledStatus == "enabled" || enabledStatus == "static"

	return status, nil
}

// getServiceStatus_Service gets service status using service command
func (m *ServiceModule) getServiceStatus_Service(name string) (*ServiceStatus, error) {
	status := &ServiceStatus{Name: name}

	// Try service status command
	cmd := exec.Command("service", name, "status")
	err := cmd.Run()

	// If exit code is 0, service is typically running
	status.Running = err == nil
	if err == nil {
		status.Status = "running"
	} else {
		status.Status = "stopped"
	}

	// Enabled status is harder to determine with service command
	// This is a simplified implementation
	status.Enabled = true // Assume enabled if service exists

	return status, nil
}

// getSysVStatus gets service status using SysV init scripts
func (m *ServiceModule) getSysVStatus(name string) (*ServiceStatus, error) {
	status := &ServiceStatus{Name: name}

	// This is a simplified implementation
	// Real implementation would check /etc/init.d/ scripts and runlevels
	status.Running = false
	status.Enabled = false
	status.Status = "unknown"

	return status, nil
}

// handleState handles service state changes
func (m *ServiceModule) handleState(manager ServiceManager, name, desiredState string, currentStatus *ServiceStatus, sleep int) (bool, string, error) {
	switch desiredState {
	case "started":
		if !currentStatus.Running {
			if err := m.startService(manager, name); err != nil {
				return false, "", err
			}
			time.Sleep(time.Duration(sleep) * time.Second)
			return true, "started", nil
		}

	case "stopped":
		if currentStatus.Running {
			if err := m.stopService(manager, name); err != nil {
				return false, "", err
			}
			time.Sleep(time.Duration(sleep) * time.Second)
			return true, "stopped", nil
		}

	case "restarted":
		if err := m.restartService(manager, name); err != nil {
			return false, "", err
		}
		time.Sleep(time.Duration(sleep) * time.Second)
		return true, "restarted", nil

	case "reloaded":
		if err := m.reloadService(manager, name); err != nil {
			return false, "", err
		}
		time.Sleep(time.Duration(sleep) * time.Second)
		return true, "reloaded", nil
	}

	return false, "", nil
}

// handleEnabled handles service enabled state changes
func (m *ServiceModule) handleEnabled(manager ServiceManager, name string, enabled interface{}, currentStatus *ServiceStatus) (bool, string, error) {
	desiredEnabled := false
	if enabledBool, ok := enabled.(bool); ok {
		desiredEnabled = enabledBool
	} else if enabledStr, ok := enabled.(string); ok {
		desiredEnabled = enabledStr == "yes" || enabledStr == "true" || enabledStr == "1"
	}

	if desiredEnabled && !currentStatus.Enabled {
		if err := m.enableService(manager, name); err != nil {
			return false, "", err
		}
		return true, "enabled", nil
	} else if !desiredEnabled && currentStatus.Enabled {
		if err := m.disableService(manager, name); err != nil {
			return false, "", err
		}
		return true, "disabled", nil
	}

	return false, "", nil
}

// Service management operations

func (m *ServiceModule) startService(manager ServiceManager, name string) error {
	switch manager {
	case ServiceManagerSystemd:
		return exec.Command("systemctl", "start", name).Run()
	case ServiceManagerService:
		return exec.Command("service", name, "start").Run()
	default:
		return fmt.Errorf("start operation not supported for service manager: %s", manager)
	}
}

func (m *ServiceModule) stopService(manager ServiceManager, name string) error {
	switch manager {
	case ServiceManagerSystemd:
		return exec.Command("systemctl", "stop", name).Run()
	case ServiceManagerService:
		return exec.Command("service", name, "stop").Run()
	default:
		return fmt.Errorf("stop operation not supported for service manager: %s", manager)
	}
}

func (m *ServiceModule) restartService(manager ServiceManager, name string) error {
	switch manager {
	case ServiceManagerSystemd:
		return exec.Command("systemctl", "restart", name).Run()
	case ServiceManagerService:
		return exec.Command("service", name, "restart").Run()
	default:
		return fmt.Errorf("restart operation not supported for service manager: %s", manager)
	}
}

func (m *ServiceModule) reloadService(manager ServiceManager, name string) error {
	switch manager {
	case ServiceManagerSystemd:
		return exec.Command("systemctl", "reload", name).Run()
	case ServiceManagerService:
		return exec.Command("service", name, "reload").Run()
	default:
		return fmt.Errorf("reload operation not supported for service manager: %s", manager)
	}
}

func (m *ServiceModule) enableService(manager ServiceManager, name string) error {
	switch manager {
	case ServiceManagerSystemd:
		return exec.Command("systemctl", "enable", name).Run()
	default:
		// Service command doesn't typically handle enable/disable
		return fmt.Errorf("enable operation not supported for service manager: %s", manager)
	}
}

func (m *ServiceModule) disableService(manager ServiceManager, name string) error {
	switch manager {
	case ServiceManagerSystemd:
		return exec.Command("systemctl", "disable", name).Run()
	default:
		return fmt.Errorf("disable operation not supported for service manager: %s", manager)
	}
}

// handleCheckMode returns appropriate result for check mode
func (m *ServiceModule) handleCheckMode(name, state string, enabled interface{}, currentStatus *ServiceStatus) (*ModuleResult, error) {
	changes := []string{}

	if state != "" {
		switch state {
		case "started":
			if !currentStatus.Running {
				changes = append(changes, "would start service")
			}
		case "stopped":
			if currentStatus.Running {
				changes = append(changes, "would stop service")
			}
		case "restarted":
			changes = append(changes, "would restart service")
		case "reloaded":
			changes = append(changes, "would reload service")
		}
	}

	if enabled != nil {
		desiredEnabled := false
		if enabledBool, ok := enabled.(bool); ok {
			desiredEnabled = enabledBool
		} else if enabledStr, ok := enabled.(string); ok {
			desiredEnabled = enabledStr == "yes" || enabledStr == "true"
		}

		if desiredEnabled && !currentStatus.Enabled {
			changes = append(changes, "would enable service")
		} else if !desiredEnabled && currentStatus.Enabled {
			changes = append(changes, "would disable service")
		}
	}

	if len(changes) > 0 {
		return ChangedResult(fmt.Sprintf("Service %s: %s", name, strings.Join(changes, ", "))), nil
	}

	return UnchangedResult(fmt.Sprintf("Service %s is already in desired state", name)), nil
}

// mapStatusToState maps service running status to Ansible state
func (m *ServiceModule) mapStatusToState(running bool) string {
	if running {
		return "started"
	}
	return "stopped"
}

