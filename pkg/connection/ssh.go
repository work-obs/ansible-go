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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHConnection implements Connection for SSH connections
type SSHConnection struct {
	*BaseConnection
	client  *ssh.Client
	session *ssh.Session
}

// NewSSHConnection creates a new SSH connection
func NewSSHConnection(config *ConnectionConfig) (Connection, error) {
	if err := ValidateConfig(config); err != nil {
		return nil, err
	}

	// Set default port if not specified
	if config.Port == 0 {
		config.Port = 22
	}

	// Set default user if not specified
	if config.User == "" {
		config.User = "root"
	}

	return &SSHConnection{
		BaseConnection: NewBaseConnection(config),
	}, nil
}

// Connect establishes the SSH connection
func (c *SSHConnection) Connect(ctx context.Context) error {
	config := c.GetConfig()

	// Build SSH client configuration
	sshConfig, err := c.buildSSHConfig()
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	// Connect to SSH server
	address := net.JoinHostPort(config.Host, strconv.Itoa(config.Port))

	// Create connection with timeout
	conn, err := net.DialTimeout("tcp", address, config.ConnectTimeout)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Perform SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, sshConfig)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SSH handshake failed: %w", err)
	}

	// Create SSH client
	c.client = ssh.NewClient(sshConn, chans, reqs)
	c.SetConnected(true)

	return nil
}

// Close closes the SSH connection
func (c *SSHConnection) Close() error {
	if c.session != nil {
		c.session.Close()
		c.session = nil
	}

	if c.client != nil {
		err := c.client.Close()
		c.client = nil
		c.SetConnected(false)
		return err
	}

	c.SetConnected(false)
	return nil
}

// GetConnectionType returns the connection type
func (c *SSHConnection) GetConnectionType() string {
	return "ssh"
}

// Execute runs a command on the remote host via SSH
func (c *SSHConnection) Execute(ctx context.Context, cmd string, stdin io.Reader) (*ExecutionResult, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	startTime := time.Now()

	// Apply become if configured
	if c.GetConfig().Become {
		cmd = c.ApplyBecome(cmd)
	}

	// Create new session for this command
	session, err := c.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up environment variables
	if env := c.GetConfig().Environment; env != nil {
		for key, value := range env {
			if err := session.Setenv(key, value); err != nil {
				// Some SSH servers don't allow environment variables
				// We'll continue but prepend them to the command
				cmd = fmt.Sprintf("export %s='%s'; %s", key, value, cmd)
			}
		}
	}

	// Set up stdin
	if stdin != nil {
		session.Stdin = stdin
	}

	// Execute command and capture output
	stdout, stderr, exitCode, err := c.runSSHCommand(session, cmd)

	duration := time.Since(startTime)

	result := &ExecutionResult{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
		Error:    err,
	}

	return result, nil
}

// runSSHCommand executes a command via SSH and returns stdout, stderr, exit code, and error
func (c *SSHConnection) runSSHCommand(session *ssh.Session, cmd string) (string, string, int, error) {
	// Create pipes for stdout and stderr
	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return "", "", -1, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := session.Start(cmd); err != nil {
		return "", "", -1, fmt.Errorf("failed to start command: %w", err)
	}

	// Read stdout and stderr concurrently
	stdoutChan := make(chan string, 1)
	stderrChan := make(chan string, 1)

	go func() {
		stdout, _ := io.ReadAll(stdoutPipe)
		stdoutChan <- string(stdout)
	}()

	go func() {
		stderr, _ := io.ReadAll(stderrPipe)
		stderrChan <- string(stderr)
	}()

	// Wait for command to complete
	err = session.Wait()

	// Get output
	stdout := <-stdoutChan
	stderr := <-stderrChan

	// Determine exit code
	exitCode := 0
	if err != nil {
		if exitError, ok := err.(*ssh.ExitError); ok {
			exitCode = exitError.ExitStatus()
		} else {
			exitCode = -1
		}
	}

	return stdout, stderr, exitCode, err
}

// PutFile copies a file from local to remote via SFTP
func (c *SSHConnection) PutFile(ctx context.Context, localPath, remotePath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Read local file
	localData, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file: %w", err)
	}

	// Get local file info for permissions
	localStat, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to get local file info: %w", err)
	}

	// Use SCP-like approach since we don't have SFTP library
	// Create remote directory if needed
	remoteDir := filepath.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		_, err := c.Execute(ctx, fmt.Sprintf("mkdir -p '%s'", remoteDir), nil)
		if err != nil {
			return fmt.Errorf("failed to create remote directory: %w", err)
		}
	}

	// Create the file content as a here-document to avoid issues with special characters
	escapedContent := strings.ReplaceAll(string(localData), "'", "'\"'\"'")
	cmd := fmt.Sprintf("cat > '%s' << 'EOF'\n%s\nEOF", remotePath, escapedContent)

	result, err := c.Execute(ctx, cmd, nil)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to write file: %s", result.Stderr)
	}

	// Set file permissions
	mode := localStat.Mode().Perm()
	_, err = c.Execute(ctx, fmt.Sprintf("chmod %o '%s'", mode, remotePath), nil)
	if err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}

// GetFile copies a file from remote to local via SCP-like approach
func (c *SSHConnection) GetFile(ctx context.Context, remotePath, localPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Read remote file content
	result, err := c.Execute(ctx, fmt.Sprintf("cat '%s'", remotePath), nil)
	if err != nil {
		return fmt.Errorf("failed to read remote file: %w", err)
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to read remote file: %s", result.Stderr)
	}

	// Create local directory if needed
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// Write to local file
	if err := os.WriteFile(localPath, []byte(result.Stdout), 0644); err != nil {
		return fmt.Errorf("failed to write local file: %w", err)
	}

	return nil
}

// FileExists checks if a file exists on the remote host
func (c *SSHConnection) FileExists(ctx context.Context, path string) (bool, error) {
	result, err := c.Execute(ctx, fmt.Sprintf("test -e '%s'", path), nil)
	if err != nil {
		return false, err
	}

	return result.ExitCode == 0, nil
}

// CreateDirectory creates a directory on the remote host
func (c *SSHConnection) CreateDirectory(ctx context.Context, path string, mode uint32) error {
	cmd := fmt.Sprintf("mkdir -p '%s' && chmod %o '%s'", path, mode, path)
	result, err := c.Execute(ctx, cmd, nil)
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to create directory: %s", result.Stderr)
	}

	return nil
}

// RemoveFile removes a file on the remote host
func (c *SSHConnection) RemoveFile(ctx context.Context, path string) error {
	result, err := c.Execute(ctx, fmt.Sprintf("rm -f '%s'", path), nil)
	if err != nil {
		return err
	}

	if result.ExitCode != 0 {
		return fmt.Errorf("failed to remove file: %s", result.Stderr)
	}

	return nil
}

// GetFileInfo gets file information from the remote host
func (c *SSHConnection) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	// Use stat command to get file information
	cmd := fmt.Sprintf("stat -c '%%n|%%s|%%Y|%%f|%%U|%%G' '%s' 2>/dev/null || stat -f '%%N|%%z|%%m|%%XP|%%Su|%%Sg' '%s'", path, path)
	result, err := c.Execute(ctx, cmd, nil)
	if err != nil {
		return nil, err
	}

	if result.ExitCode != 0 {
		return nil, fmt.Errorf("file not found: %s", path)
	}

	// Parse stat output (this is simplified - real implementation would be more robust)
	parts := strings.Split(strings.TrimSpace(result.Stdout), "|")
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid stat output")
	}

	fileInfo := &FileInfo{
		Name:  filepath.Base(parts[0]),
		Owner: parts[4],
		Group: parts[5],
	}

	// Parse size
	if size, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
		fileInfo.Size = size
	}

	// Parse modification time
	if mtime, err := strconv.ParseInt(parts[2], 10, 64); err == nil {
		fileInfo.ModTime = time.Unix(mtime, 0)
	}

	// Parse mode and determine if directory
	if mode, err := strconv.ParseUint(parts[3], 16, 32); err == nil {
		fileInfo.Mode = uint32(mode) & 0777
		fileInfo.IsDir = (mode & 0x4000) != 0 // S_IFDIR
	}

	return fileInfo, nil
}

// buildSSHConfig builds the SSH client configuration
func (c *SSHConnection) buildSSHConfig() (*ssh.ClientConfig, error) {
	config := c.GetConfig()

	sshConfig := &ssh.ClientConfig{
		User:            config.User,
		Timeout:         config.ConnectTimeout,
		HostKeyCallback: c.buildHostKeyCallback(),
	}

	// Set up authentication methods
	authMethods, err := c.buildAuthMethods()
	if err != nil {
		return nil, fmt.Errorf("failed to build auth methods: %w", err)
	}

	sshConfig.Auth = authMethods

	return sshConfig, nil
}

// buildHostKeyCallback builds the host key callback
func (c *SSHConnection) buildHostKeyCallback() ssh.HostKeyCallback {
	config := c.GetConfig()

	if !config.HostKeyChecking {
		return ssh.InsecureIgnoreHostKey()
	}

	// In a real implementation, this would check against known_hosts
	// For now, we'll use the insecure callback
	return ssh.InsecureIgnoreHostKey()
}

// buildAuthMethods builds the authentication methods
func (c *SSHConnection) buildAuthMethods() ([]ssh.AuthMethod, error) {
	config := c.GetConfig()
	var authMethods []ssh.AuthMethod

	// Password authentication
	if config.Password != "" {
		authMethods = append(authMethods, ssh.Password(config.Password))
	}

	// Public key authentication
	if config.PrivateKey != "" || config.PrivateKeyFile != "" {
		keyAuth, err := c.buildKeyAuth()
		if err != nil {
			return nil, fmt.Errorf("failed to build key auth: %w", err)
		}
		if keyAuth != nil {
			authMethods = append(authMethods, keyAuth)
		}
	}

	// If no explicit auth methods, try default key files
	if len(authMethods) == 0 {
		if keyAuth := c.tryDefaultKeys(); keyAuth != nil {
			authMethods = append(authMethods, keyAuth)
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication methods available")
	}

	return authMethods, nil
}

// buildKeyAuth builds public key authentication
func (c *SSHConnection) buildKeyAuth() (ssh.AuthMethod, error) {
	config := c.GetConfig()

	var keyData []byte
	var err error

	if config.PrivateKey != "" {
		keyData = []byte(config.PrivateKey)
	} else if config.PrivateKeyFile != "" {
		keyData, err = os.ReadFile(config.PrivateKeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file: %w", err)
		}
	} else {
		return nil, nil
	}

	// Parse private key
	signer, err := c.parsePrivateKey(keyData)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return ssh.PublicKeys(signer), nil
}

// parsePrivateKey parses a private key
func (c *SSHConnection) parsePrivateKey(keyData []byte) (ssh.Signer, error) {
	// Try parsing as is first
	signer, err := ssh.ParsePrivateKey(keyData)
	if err == nil {
		return signer, nil
	}

	// If that fails, try parsing with password
	config := c.GetConfig()
	if config.Password != "" {
		signer, err := ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(config.Password))
		if err == nil {
			return signer, nil
		}
	}

	// Try different key formats
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return ssh.NewSignerFromKey(key)

	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		return ssh.NewSignerFromKey(key)

	default:
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
}

// tryDefaultKeys tries to load default SSH key files
func (c *SSHConnection) tryDefaultKeys() ssh.AuthMethod {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	keyFiles := []string{
		filepath.Join(homeDir, ".ssh", "id_rsa"),
		filepath.Join(homeDir, ".ssh", "id_ecdsa"),
		filepath.Join(homeDir, ".ssh", "id_ed25519"),
	}

	for _, keyFile := range keyFiles {
		if keyData, err := os.ReadFile(keyFile); err == nil {
			if signer, err := c.parsePrivateKey(keyData); err == nil {
				return ssh.PublicKeys(signer)
			}
		}
	}

	return nil
}

// GenerateSSHKeyPair generates a new SSH key pair for testing
func GenerateSSHKeyPair() (privateKey, publicKey string, err error) {
	// Generate RSA private key
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	// Encode private key
	privKeyPEM := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(rsaKey),
	}
	privateKey = string(pem.EncodeToMemory(privKeyPEM))

	// Generate public key
	pub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	if err != nil {
		return "", "", err
	}
	publicKey = string(ssh.MarshalAuthorizedKey(pub))

	return privateKey, publicKey, nil
}