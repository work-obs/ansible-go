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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConnectionConfig tests the connection configuration
func TestConnectionConfig(t *testing.T) {
	config := &ConnectionConfig{
		Host: "localhost",
		Port: 22,
		User: "testuser",
	}

	if err := ValidateConfig(config); err != nil {
		t.Errorf("Valid config should not fail validation: %v", err)
	}

	// Test empty host
	emptyConfig := &ConnectionConfig{}
	if err := ValidateConfig(emptyConfig); err == nil {
		t.Error("Empty host should fail validation")
	}

	// Test default timeouts
	config = &ConnectionConfig{Host: "localhost"}
	if err := ValidateConfig(config); err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}

	if config.ConnectTimeout != 10*time.Second {
		t.Errorf("Expected default connect timeout 10s, got %v", config.ConnectTimeout)
	}
}

// TestDefaultConnectionFactory tests the connection factory
func TestDefaultConnectionFactory(t *testing.T) {
	factory := NewDefaultConnectionFactory()

	// Test supported types
	types := factory.GetSupportedTypes()
	expectedTypes := []string{"local", "ssh", "paramiko", "smart"}

	for _, expectedType := range expectedTypes {
		found := false
		for _, actualType := range types {
			if actualType == expectedType {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected connection type %s not found", expectedType)
		}
	}

	// Test creating local connection
	config := &ConnectionConfig{Host: "localhost"}
	conn, err := factory.CreateConnection("local", config)
	if err != nil {
		t.Errorf("Failed to create local connection: %v", err)
	}

	if conn.GetConnectionType() != "local" {
		t.Errorf("Expected connection type 'local', got '%s'", conn.GetConnectionType())
	}

	// Test unsupported connection type
	_, err = factory.CreateConnection("invalid", config)
	if err == nil {
		t.Error("Should fail for unsupported connection type")
	}
}

// TestConnectionManager tests the connection manager
func TestConnectionManager(t *testing.T) {
	manager := NewConnectionManager(nil) // Use default factory

	config := &ConnectionConfig{Host: "localhost"}

	// Test getting connection
	conn1, err := manager.GetConnection("localhost", "local", config)
	if err != nil {
		t.Errorf("Failed to get connection: %v", err)
	}

	// Test getting same connection again (should reuse)
	conn2, err := manager.GetConnection("localhost", "local", config)
	if err != nil {
		t.Errorf("Failed to get connection: %v", err)
	}

	// Should be the same connection object (assuming it's connected)
	if err := conn1.Connect(context.Background()); err != nil {
		t.Errorf("Failed to connect: %v", err)
	}

	// Test closing specific connection
	if err := manager.CloseConnection("localhost", "local"); err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}

	// Test closing all connections
	if err := manager.CloseAllConnections(); err != nil {
		t.Errorf("Failed to close all connections: %v", err)
	}

	// Active connections should be empty
	active := manager.GetActiveConnections()
	if len(active) != 0 {
		t.Errorf("Expected 0 active connections, got %d", len(active))
	}

	// Clean up
	conn2.Close()
}

// TestLocalConnection tests the local connection
func TestLocalConnection(t *testing.T) {
	config := &ConnectionConfig{Host: "localhost"}
	conn, err := NewLocalConnection(config)
	if err != nil {
		t.Fatalf("Failed to create local connection: %v", err)
	}

	ctx := context.Background()

	// Test connection
	if err := conn.Connect(ctx); err != nil {
		t.Errorf("Failed to connect: %v", err)
	}

	if !conn.IsConnected() {
		t.Error("Connection should be connected")
	}

	if conn.GetHost() != "localhost" {
		t.Errorf("Expected host 'localhost', got '%s'", conn.GetHost())
	}

	if conn.GetConnectionType() != "local" {
		t.Errorf("Expected connection type 'local', got '%s'", conn.GetConnectionType())
	}

	// Test command execution
	result, err := conn.Execute(ctx, "echo 'test'", nil)
	if err != nil {
		t.Errorf("Failed to execute command: %v", err)
	}

	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	if !strings.Contains(result.Stdout, "test") {
		t.Errorf("Expected 'test' in stdout, got: %s", result.Stdout)
	}

	// Test file operations
	tempDir, err := os.MkdirTemp("", "ansible_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFile := filepath.Join(tempDir, "test.txt")
	testContent := "Hello, World!"

	// Create a test file
	if err := os.WriteFile(testFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test FileExists
	exists, err := conn.FileExists(ctx, testFile)
	if err != nil {
		t.Errorf("FileExists failed: %v", err)
	}
	if !exists {
		t.Error("File should exist")
	}

	// Test GetFileInfo
	fileInfo, err := conn.GetFileInfo(ctx, testFile)
	if err != nil {
		t.Errorf("GetFileInfo failed: %v", err)
	}
	if fileInfo.Size != int64(len(testContent)) {
		t.Errorf("Expected file size %d, got %d", len(testContent), fileInfo.Size)
	}

	// Test PutFile (copy to another location)
	copyFile := filepath.Join(tempDir, "copy.txt")
	if err := conn.PutFile(ctx, testFile, copyFile); err != nil {
		t.Errorf("PutFile failed: %v", err)
	}

	// Verify copy
	copyContent, err := os.ReadFile(copyFile)
	if err != nil {
		t.Errorf("Failed to read copied file: %v", err)
	}
	if string(copyContent) != testContent {
		t.Errorf("Copied content mismatch. Expected '%s', got '%s'", testContent, string(copyContent))
	}

	// Test GetFile
	getFile := filepath.Join(tempDir, "get.txt")
	if err := conn.GetFile(ctx, copyFile, getFile); err != nil {
		t.Errorf("GetFile failed: %v", err)
	}

	// Test CreateDirectory
	testDir := filepath.Join(tempDir, "testdir")
	if err := conn.CreateDirectory(ctx, testDir, 0755); err != nil {
		t.Errorf("CreateDirectory failed: %v", err)
	}

	if _, err := os.Stat(testDir); os.IsNotExist(err) {
		t.Error("Directory was not created")
	}

	// Test RemoveFile
	if err := conn.RemoveFile(ctx, testFile); err != nil {
		t.Errorf("RemoveFile failed: %v", err)
	}

	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("File was not removed")
	}

	// Test close
	if err := conn.Close(); err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}

	if conn.IsConnected() {
		t.Error("Connection should not be connected after close")
	}
}

// TestBaseConnection tests the base connection functionality
func TestBaseConnection(t *testing.T) {
	config := &ConnectionConfig{
		Host:   "testhost",
		Become: true,
		BecomeMethod: "sudo",
		BecomeUser:   "root",
	}

	base := NewBaseConnection(config)

	if base.GetHost() != "testhost" {
		t.Errorf("Expected host 'testhost', got '%s'", base.GetHost())
	}

	if base.IsConnected() {
		t.Error("New connection should not be connected")
	}

	// Test become application
	cmd := "whoami"
	becomeCmd := base.ApplyBecome(cmd)
	expectedCmd := "sudo -u root whoami"

	if becomeCmd != expectedCmd {
		t.Errorf("Expected become command '%s', got '%s'", expectedCmd, becomeCmd)
	}

	// Test different become methods
	config.BecomeMethod = "su"
	becomeCmd = base.ApplyBecome(cmd)
	expectedCmd = "su - root -c 'whoami'"

	if becomeCmd != expectedCmd {
		t.Errorf("Expected su command '%s', got '%s'", expectedCmd, becomeCmd)
	}

	// Test no become
	config.Become = false
	becomeCmd = base.ApplyBecome(cmd)

	if becomeCmd != cmd {
		t.Errorf("Expected original command '%s', got '%s'", cmd, becomeCmd)
	}
}

// TestSSHConnection tests SSH connection creation (without actual connection)
func TestSSHConnection(t *testing.T) {
	config := &ConnectionConfig{
		Host: "example.com",
		Port: 22,
		User: "testuser",
	}

	conn, err := NewSSHConnection(config)
	if err != nil {
		t.Fatalf("Failed to create SSH connection: %v", err)
	}

	if conn.GetConnectionType() != "ssh" {
		t.Errorf("Expected connection type 'ssh', got '%s'", conn.GetConnectionType())
	}

	if conn.GetHost() != "example.com" {
		t.Errorf("Expected host 'example.com', got '%s'", conn.GetHost())
	}

	// Test default port setting
	if conn.(*SSHConnection).GetConfig().Port != 22 {
		t.Errorf("Expected default port 22, got %d", conn.(*SSHConnection).GetConfig().Port)
	}

	// Test default user setting
	if conn.(*SSHConnection).GetConfig().User != "testuser" {
		t.Errorf("Expected user 'testuser', got '%s'", conn.(*SSHConnection).GetConfig().User)
	}
}

// TestSSHKeyGeneration tests SSH key pair generation
func TestSSHKeyGeneration(t *testing.T) {
	privateKey, publicKey, err := GenerateSSHKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate SSH key pair: %v", err)
	}

	if privateKey == "" {
		t.Error("Private key should not be empty")
	}

	if publicKey == "" {
		t.Error("Public key should not be empty")
	}

	if !strings.Contains(privateKey, "BEGIN RSA PRIVATE KEY") {
		t.Error("Private key should contain RSA private key header")
	}

	if !strings.HasPrefix(publicKey, "ssh-rsa") {
		t.Error("Public key should start with 'ssh-rsa'")
	}
}

// TestCommandExecution tests command execution with different scenarios
func TestCommandExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping command execution test in short mode")
	}

	config := &ConnectionConfig{Host: "localhost"}
	conn, err := NewLocalConnection(config)
	if err != nil {
		t.Fatalf("Failed to create local connection: %v", err)
	}

	ctx := context.Background()
	if err := conn.Connect(ctx); err != nil {
		t.Errorf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Test successful command
	result, err := conn.Execute(ctx, "echo success", nil)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", result.ExitCode)
	}

	// Test command with non-zero exit code
	result, err = conn.Execute(ctx, "exit 1", nil)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}
	if result.ExitCode != 1 {
		t.Errorf("Expected exit code 1, got %d", result.ExitCode)
	}

	// Test command with stderr
	result, err = conn.Execute(ctx, "echo 'error message' >&2", nil)
	if err != nil {
		t.Errorf("Command execution failed: %v", err)
	}
	if !strings.Contains(result.Stderr, "error message") {
		t.Errorf("Expected 'error message' in stderr, got: %s", result.Stderr)
	}
}

// TestSmartConnection tests the smart connection type
func TestSmartConnection(t *testing.T) {
	factory := NewDefaultConnectionFactory()

	// Test localhost should use local connection
	config := &ConnectionConfig{Host: "localhost"}
	conn, err := factory.CreateConnection("smart", config)
	if err != nil {
		t.Errorf("Failed to create smart connection: %v", err)
	}

	if conn.GetConnectionType() != "local" {
		t.Errorf("Smart connection for localhost should be local, got %s", conn.GetConnectionType())
	}

	// Test remote host should use SSH connection
	config = &ConnectionConfig{Host: "remote.example.com", User: "test"}
	conn, err = factory.CreateConnection("smart", config)
	if err != nil {
		t.Errorf("Failed to create smart connection: %v", err)
	}

	if conn.GetConnectionType() != "ssh" {
		t.Errorf("Smart connection for remote host should be ssh, got %s", conn.GetConnectionType())
	}
}