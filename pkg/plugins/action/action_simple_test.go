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
	"testing"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// TestBasicActionPlugin tests basic action plugin functionality
func TestBasicActionPlugin(t *testing.T) {
	registry := NewActionPluginRegistry()

	// Test that registry is created properly
	if registry == nil {
		t.Error("Registry should not be nil")
	}

	// Test command plugin creation
	commandPlugin := NewCommandActionPlugin()
	if commandPlugin.Name() != "command" {
		t.Errorf("Expected plugin name 'command', got '%s'", commandPlugin.Name())
	}

	// Test setup plugin creation
	setupPlugin := NewSetupActionPlugin()
	if setupPlugin.Name() != "setup" {
		t.Errorf("Expected plugin name 'setup', got '%s'", setupPlugin.Name())
	}

	// Test plugin registration
	registry.Register("test-command", func() plugins.ActionPlugin {
		return NewCommandActionPlugin()
	})

	if !registry.Exists("test-command") {
		t.Error("Plugin should exist after registration")
	}

	// Test getting registered plugin
	plugin, err := registry.Get("test-command")
	if err != nil {
		t.Errorf("Failed to get registered plugin: %v", err)
	}

	if plugin.Name() != "command" {
		t.Errorf("Expected plugin name 'command', got '%s'", plugin.Name())
	}
}

// TestActionPluginExecution tests basic plugin execution
func TestActionPluginExecution(t *testing.T) {
	plugin := NewSetupActionPlugin()

	// Create context
	ctx := context.Background()
	actionCtx := &plugins.ActionContext{}
	actionCtx.Args = map[string]interface{}{
		"gather_subset": []string{"min"},
	}

	// Execute plugin
	result, err := plugin.Run(ctx, actionCtx)
	if err != nil {
		t.Errorf("Plugin execution failed: %v", err)
	}

	if result.Failed {
		t.Errorf("Plugin failed: %s", result.Message)
	}

	// Check that facts were gathered
	if result.Results == nil {
		t.Error("Expected results to be returned")
	}
}