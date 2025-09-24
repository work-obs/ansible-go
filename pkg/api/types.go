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

package api

import (
	"time"
)

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

// PlaybookExecutionRequest represents a playbook execution request
type PlaybookExecutionRequest struct {
	PlaybookPath        string                 `json:"playbook_path" binding:"required"`
	Inventory           string                 `json:"inventory,omitempty"`
	Limit               string                 `json:"limit,omitempty"`
	Tags                []string               `json:"tags,omitempty"`
	SkipTags            []string               `json:"skip_tags,omitempty"`
	ExtraVars           map[string]interface{} `json:"extra_vars,omitempty"`
	VaultPasswordFile   string                 `json:"vault_password_file,omitempty"`
	Check               bool                   `json:"check,omitempty"`
	Diff                bool                   `json:"diff,omitempty"`
	Verbose             int                    `json:"verbose,omitempty"`
}

// PlaybookExecutionResponse represents a playbook execution response
type PlaybookExecutionResponse struct {
	ExecutionID string `json:"execution_id"`
}

// PlaybookResult represents the result of a playbook execution
type PlaybookResult struct {
	ExecutionID string         `json:"execution_id"`
	Status      string         `json:"status"` // running, completed, failed, cancelled
	StartTime   time.Time      `json:"start_time"`
	EndTime     *time.Time     `json:"end_time,omitempty"`
	Plays       []PlayResult   `json:"plays"`
	Stats       PlaybookStats  `json:"stats"`
}

// PlayResult represents the result of a single play
type PlayResult struct {
	Name  string       `json:"name"`
	Hosts []string     `json:"hosts"`
	Tasks []TaskResult `json:"tasks"`
}

// TaskResult represents the result of a single task
type TaskResult struct {
	Name      string                 `json:"name"`
	Action    string                 `json:"action"`
	Host      string                 `json:"host"`
	Status    string                 `json:"status"` // ok, changed, failed, skipped, unreachable
	Result    map[string]interface{} `json:"result"`
	StartTime time.Time              `json:"start_time"`
	EndTime   time.Time              `json:"end_time"`
}

// PlaybookStats represents overall playbook execution statistics
type PlaybookStats struct {
	OK          int `json:"ok"`
	Changed     int `json:"changed"`
	Failed      int `json:"failed"`
	Skipped     int `json:"skipped"`
	Unreachable int `json:"unreachable"`
}

// ModuleExecutionRequest represents a module execution request
type ModuleExecutionRequest struct {
	ModuleName    string                 `json:"module_name" binding:"required"`
	HostPattern   string                 `json:"host_pattern" binding:"required"`
	ModuleArgs    map[string]interface{} `json:"module_args,omitempty"`
	Inventory     string                 `json:"inventory,omitempty"`
	ExtraVars     map[string]interface{} `json:"extra_vars,omitempty"`
	Become        bool                   `json:"become,omitempty"`
	BecomeMethod  string                 `json:"become_method,omitempty"`
	BecomeUser    string                 `json:"become_user,omitempty"`
	Check         bool                   `json:"check,omitempty"`
	Verbose       int                    `json:"verbose,omitempty"`
}

// ModuleResult represents the result of a module execution
type ModuleResult struct {
	ExecutionID string                `json:"execution_id"`
	Hosts       map[string]HostResult `json:"hosts"`
}

// HostResult represents the result for a single host
type HostResult struct {
	Status string                 `json:"status"` // ok, changed, failed, skipped, unreachable
	Result map[string]interface{} `json:"result"`
	Stderr string                 `json:"stderr,omitempty"`
	Stdout string                 `json:"stdout,omitempty"`
}

// Inventory represents an Ansible inventory structure
type Inventory struct {
	All InventoryGroup `json:"all"`
}

// InventoryGroup represents a group in the inventory
type InventoryGroup struct {
	Hosts    []string                    `json:"hosts,omitempty"`
	Children map[string]*InventoryGroup  `json:"children,omitempty"`
	Vars     map[string]interface{}      `json:"vars,omitempty"`
}

// HostListResponse represents a response containing a list of hosts
type HostListResponse struct {
	Hosts []string `json:"hosts"`
}

// GroupListResponse represents a response containing a list of groups
type GroupListResponse struct {
	Groups []string `json:"groups"`
}

// Configuration represents Ansible configuration
type Configuration struct {
	Config map[string]interface{} `json:"config"`
}

// PluginInfo represents information about a plugin
type PluginInfo struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Author      []string `json:"author"`
	Version     string   `json:"version"`
	Path        string   `json:"path"`
}

// PluginListResponse represents a response containing a list of plugins
type PluginListResponse struct {
	Plugins []PluginInfo `json:"plugins"`
}

// RouterConfigResponse represents a router configuration response
type RouterConfigResponse struct {
	Config map[string]interface{} `json:"config"`
}

// RouterConfigUpdateRequest represents a router configuration update request
type RouterConfigUpdateRequest struct {
	Config map[string]interface{} `json:"config" binding:"required"`
}

// RouterConfigUpdateResponse represents a router configuration update response
type RouterConfigUpdateResponse struct {
	Status string `json:"status"`
}

// HealthResponse represents a health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
}