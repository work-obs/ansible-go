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

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func TestNewManager(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}

	if manager.fs != fs {
		t.Error("Expected filesystem to be set correctly")
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	err := manager.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	config := manager.GetConfig()

	// Test some default values
	if config.RemotePort != 22 {
		t.Errorf("Expected remote port 22, got %d", config.RemotePort)
	}

	if config.BecomeMethod != "sudo" {
		t.Errorf("Expected become method 'sudo', got '%s'", config.BecomeMethod)
	}

	if config.BecomeUser != "root" {
		t.Errorf("Expected become user 'root', got '%s'", config.BecomeUser)
	}

	if config.Timeout != 10*time.Second {
		t.Errorf("Expected timeout 10s, got %v", config.Timeout)
	}
}

func TestLoadConfig_YAML(t *testing.T) {
	fs := afero.NewMemMapFs()

	// Create a YAML config file
	yamlConfig := `
remote_user: testuser
remote_port: 2222
timeout: 30s
become: true
become_method: su
`

	err := afero.WriteFile(fs, "ansible.yaml", []byte(yamlConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	manager := NewManager(fs)
	err = manager.LoadConfigFromData([]byte(yamlConfig), "yaml")
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	config := manager.GetConfig()

	if config.RemoteUser != "testuser" {
		t.Errorf("Expected remote user 'testuser', got '%s'", config.RemoteUser)
	}

	if config.RemotePort != 2222 {
		t.Errorf("Expected remote port 2222, got %d", config.RemotePort)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", config.Timeout)
	}

	if !config.Become {
		t.Error("Expected become to be true")
	}

	if config.BecomeMethod != "su" {
		t.Errorf("Expected become method 'su', got '%s'", config.BecomeMethod)
	}
}

func TestLoadConfig_EnvironmentVariables(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	// Set environment variables
	os.Setenv("ANSIBLE_REMOTE_USER", "envuser")
	os.Setenv("ANSIBLE_REMOTE_PORT", "3333")
	defer func() {
		os.Unsetenv("ANSIBLE_REMOTE_USER")
		os.Unsetenv("ANSIBLE_REMOTE_PORT")
	}()

	err := manager.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	config := manager.GetConfig()

	if config.RemoteUser != "envuser" {
		t.Errorf("Expected remote user 'envuser', got '%s'", config.RemoteUser)
	}

	if config.RemotePort != 3333 {
		t.Errorf("Expected remote port 3333, got %d", config.RemotePort)
	}
}

func TestExpandPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "absolute path",
			input:    "/etc/ansible/hosts",
			expected: "/etc/ansible/hosts",
		},
		{
			name:     "relative path",
			input:    "hosts",
			expected: "hosts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandPath(tt.input)
			if result != tt.expected {
				t.Errorf("expandPath(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExpandPath_HomeDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot determine home directory")
	}

	result := expandPath("~/.ansible/hosts")
	expected := filepath.Join(home, ".ansible/hosts")

	if result != expected {
		t.Errorf("expandPath('~/.ansible/hosts') = %s, want %s", result, expected)
	}
}

func TestExpandPaths(t *testing.T) {
	input := []string{"/etc/ansible", "~/.ansible", "roles"}
	result := expandPaths(input)

	if len(result) != len(input) {
		t.Errorf("Expected %d paths, got %d", len(input), len(result))
	}

	// First path should be unchanged
	if result[0] != "/etc/ansible" {
		t.Errorf("Expected first path '/etc/ansible', got '%s'", result[0])
	}

	// Third path should be unchanged
	if result[2] != "roles" {
		t.Errorf("Expected third path 'roles', got '%s'", result[2])
	}
}

func TestGetCurrentUser(t *testing.T) {
	// Set a test environment variable
	os.Setenv("USER", "testuser")
	defer os.Unsetenv("USER")

	user := getCurrentUser()
	if user != "testuser" {
		t.Errorf("Expected user 'testuser', got '%s'", user)
	}
}

func TestGetCurrentUser_Fallback(t *testing.T) {
	// Unset all user environment variables
	originalUser := os.Getenv("USER")
	originalUsername := os.Getenv("USERNAME")

	os.Unsetenv("USER")
	os.Unsetenv("USERNAME")

	defer func() {
		if originalUser != "" {
			os.Setenv("USER", originalUser)
		}
		if originalUsername != "" {
			os.Setenv("USERNAME", originalUsername)
		}
	}()

	user := getCurrentUser()
	if user != "ansible" {
		t.Errorf("Expected fallback user 'ansible', got '%s'", user)
	}
}

func TestIsConfigNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		error    string
		expected bool
	}{
		{
			name:     "not found error",
			error:    "Config File \"ansible\" Not Found",
			expected: true,
		},
		{
			name:     "no such file error",
			error:    "open ansible.yaml: no such file or directory",
			expected: true,
		},
		{
			name:     "other error",
			error:    "permission denied",
			expected: false,
		},
		{
			name:     "empty error",
			error:    "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fmt.Errorf("%s", tt.error)
			result := isConfigNotFoundError(err)
			if result != tt.expected {
				t.Errorf("isConfigNotFoundError(%s) = %v, want %v", tt.error, result, tt.expected)
			}
		})
	}
}

func TestGetValue_SetValue(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	err := manager.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test setting and getting a value
	key := "test_key"
	value := "test_value"

	manager.SetValue(key, value)
	result := manager.GetValue(key)

	if result != value {
		t.Errorf("Expected value '%s', got '%v'", value, result)
	}
}