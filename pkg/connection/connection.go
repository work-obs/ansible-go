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
	"time"
)

// Connection represents a connection to a remote host
type Connection interface {
	// Connect establishes the connection to the remote host
	Connect(ctx context.Context) error

	// Close closes the connection
	Close() error

	// Execute runs a command on the remote host
	Execute(ctx context.Context, cmd string, stdin io.Reader) (*ExecutionResult, error)

	// PutFile copies a file from local to remote
	PutFile(ctx context.Context, localPath, remotePath string) error

	// GetFile copies a file from remote to local
	GetFile(ctx context.Context, remotePath, localPath string) error

	// FileExists checks if a file exists on the remote host
	FileExists(ctx context.Context, path string) (bool, error)

	// CreateDirectory creates a directory on the remote host
	CreateDirectory(ctx context.Context, path string, mode uint32) error

	// RemoveFile removes a file on the remote host
	RemoveFile(ctx context.Context, path string) error

	// GetFileInfo gets file information from the remote host
	GetFileInfo(ctx context.Context, path string) (*FileInfo, error)

	// IsConnected returns true if the connection is active
	IsConnected() bool

	// GetHost returns the hostname/IP this connection is for
	GetHost() string

	// GetConnectionType returns the type of connection
	GetConnectionType() string
}

// ExecutionResult represents the result of executing a command
type ExecutionResult struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration time.Duration `json:"duration"`
	Error    error  `json:"error,omitempty"`
}

// FileInfo represents file information from a remote host
type FileInfo struct {
	Name    string      `json:"name"`
	Size    int64       `json:"size"`
	Mode    uint32      `json:"mode"`
	ModTime time.Time   `json:"mod_time"`
	IsDir   bool        `json:"is_dir"`
	Owner   string      `json:"owner,omitempty"`
	Group   string      `json:"group,omitempty"`
}

// ConnectionConfig holds configuration for connections
type ConnectionConfig struct {
	// Common connection settings
	Host            string            `json:"host"`
	Port            int               `json:"port,omitempty"`
	User            string            `json:"user,omitempty"`
	Password        string            `json:"password,omitempty"`
	Timeout         time.Duration     `json:"timeout,omitempty"`
	ConnectTimeout  time.Duration     `json:"connect_timeout,omitempty"`

	// SSH-specific settings
	PrivateKey      string            `json:"private_key,omitempty"`
	PrivateKeyFile  string            `json:"private_key_file,omitempty"`
	HostKeyChecking bool              `json:"host_key_checking"`
	ProxyCommand    string            `json:"proxy_command,omitempty"`

	// Become settings
	Become         bool              `json:"become,omitempty"`
	BecomeMethod   string            `json:"become_method,omitempty"`
	BecomeUser     string            `json:"become_user,omitempty"`
	BecomePassword string            `json:"become_password,omitempty"`

	// Additional options
	Environment    map[string]string `json:"environment,omitempty"`
	Extra          map[string]interface{} `json:"extra,omitempty"`
}

// ConnectionFactory creates connections of different types
type ConnectionFactory interface {
	CreateConnection(connectionType string, config *ConnectionConfig) (Connection, error)
	GetSupportedTypes() []string
}

// DefaultConnectionFactory implements ConnectionFactory
type DefaultConnectionFactory struct {
	creators map[string]ConnectionCreator
}

// ConnectionCreator is a function that creates a specific type of connection
type ConnectionCreator func(config *ConnectionConfig) (Connection, error)

// NewDefaultConnectionFactory creates a new default connection factory
func NewDefaultConnectionFactory() *DefaultConnectionFactory {
	factory := &DefaultConnectionFactory{
		creators: make(map[string]ConnectionCreator),
	}

	// Register default connection types
	factory.RegisterConnection("local", NewLocalConnection)
	factory.RegisterConnection("ssh", NewSSHConnection)
	factory.RegisterConnection("paramiko", NewSSHConnection) // Alias for SSH
	factory.RegisterConnection("smart", func(config *ConnectionConfig) (Connection, error) {
		// Smart connection chooses the best connection type
		if config.Host == "localhost" || config.Host == "127.0.0.1" || config.Host == "" {
			return NewLocalConnection(config)
		}
		return NewSSHConnection(config)
	})

	return factory
}

// RegisterConnection registers a new connection type
func (f *DefaultConnectionFactory) RegisterConnection(connType string, creator ConnectionCreator) {
	f.creators[connType] = creator
}

// CreateConnection creates a connection of the specified type
func (f *DefaultConnectionFactory) CreateConnection(connectionType string, config *ConnectionConfig) (Connection, error) {
	creator, exists := f.creators[connectionType]
	if !exists {
		return nil, fmt.Errorf("unsupported connection type: %s", connectionType)
	}

	return creator(config)
}

// GetSupportedTypes returns a list of supported connection types
func (f *DefaultConnectionFactory) GetSupportedTypes() []string {
	types := make([]string, 0, len(f.creators))
	for connType := range f.creators {
		types = append(types, connType)
	}
	return types
}

// ConnectionManager manages multiple connections
type ConnectionManager struct {
	factory     ConnectionFactory
	connections map[string]Connection
	configs     map[string]*ConnectionConfig
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(factory ConnectionFactory) *ConnectionManager {
	if factory == nil {
		factory = NewDefaultConnectionFactory()
	}

	return &ConnectionManager{
		factory:     factory,
		connections: make(map[string]Connection),
		configs:     make(map[string]*ConnectionConfig),
	}
}

// GetConnection gets or creates a connection for the specified host
func (m *ConnectionManager) GetConnection(host string, connectionType string, config *ConnectionConfig) (Connection, error) {
	key := fmt.Sprintf("%s:%s", host, connectionType)

	// Check if we already have a connection
	if conn, exists := m.connections[key]; exists && conn.IsConnected() {
		return conn, nil
	}

	// Create new connection
	conn, err := m.factory.CreateConnection(connectionType, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection: %w", err)
	}

	// Store the connection and config
	m.connections[key] = conn
	m.configs[key] = config

	return conn, nil
}

// CloseConnection closes a specific connection
func (m *ConnectionManager) CloseConnection(host string, connectionType string) error {
	key := fmt.Sprintf("%s:%s", host, connectionType)

	if conn, exists := m.connections[key]; exists {
		err := conn.Close()
		delete(m.connections, key)
		delete(m.configs, key)
		return err
	}

	return nil
}

// CloseAllConnections closes all active connections
func (m *ConnectionManager) CloseAllConnections() error {
	var lastError error

	for key, conn := range m.connections {
		if err := conn.Close(); err != nil {
			lastError = err
		}
		delete(m.connections, key)
		delete(m.configs, key)
	}

	return lastError
}

// GetActiveConnections returns a list of active connections
func (m *ConnectionManager) GetActiveConnections() []string {
	active := make([]string, 0)

	for key, conn := range m.connections {
		if conn.IsConnected() {
			active = append(active, key)
		}
	}

	return active
}

// BaseConnection provides common functionality for all connection types
type BaseConnection struct {
	config    *ConnectionConfig
	host      string
	connected bool
}

// NewBaseConnection creates a new base connection
func NewBaseConnection(config *ConnectionConfig) *BaseConnection {
	return &BaseConnection{
		config:    config,
		host:      config.Host,
		connected: false,
	}
}

// GetHost returns the hostname/IP this connection is for
func (c *BaseConnection) GetHost() string {
	return c.host
}

// IsConnected returns true if the connection is active
func (c *BaseConnection) IsConnected() bool {
	return c.connected
}

// SetConnected sets the connection status
func (c *BaseConnection) SetConnected(connected bool) {
	c.connected = connected
}

// GetConfig returns the connection configuration
func (c *BaseConnection) GetConfig() *ConnectionConfig {
	return c.config
}

// ApplyBecome applies become (sudo/su) to a command if configured
func (c *BaseConnection) ApplyBecome(cmd string) string {
	if !c.config.Become {
		return cmd
	}

	becomeMethod := c.config.BecomeMethod
	if becomeMethod == "" {
		becomeMethod = "sudo"
	}

	switch becomeMethod {
	case "sudo":
		becomeCmd := "sudo"
		if c.config.BecomeUser != "" {
			becomeCmd += " -u " + c.config.BecomeUser
		}
		return becomeCmd + " " + cmd

	case "su":
		if c.config.BecomeUser != "" {
			return "su - " + c.config.BecomeUser + " -c '" + cmd + "'"
		}
		return "su - -c '" + cmd + "'"

	case "doas":
		becomeCmd := "doas"
		if c.config.BecomeUser != "" {
			becomeCmd += " -u " + c.config.BecomeUser
		}
		return becomeCmd + " " + cmd

	default:
		// For unknown become methods, just prepend the method name
		return becomeMethod + " " + cmd
	}
}

// ValidateConfig validates the connection configuration
func ValidateConfig(config *ConnectionConfig) error {
	if config == nil {
		return fmt.Errorf("connection configuration is required")
	}

	if config.Host == "" {
		return fmt.Errorf("host is required")
	}

	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	if config.ConnectTimeout == 0 {
		config.ConnectTimeout = 10 * time.Second
	}

	return nil
}