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
	"strings"
	"time"

	"github.com/spf13/viper"
	"github.com/spf13/afero"
)

// Config represents the complete Ansible configuration
type Config struct {
	// Core settings
	PrivateKeyFile     string        `mapstructure:"private_key_file"`
	HostKeyChecking    bool          `mapstructure:"host_key_checking"`
	Timeout            time.Duration `mapstructure:"timeout"`
	RemoteUser         string        `mapstructure:"remote_user"`
	AskPass            bool          `mapstructure:"ask_pass"`
	AskSudoPass        bool          `mapstructure:"ask_sudo_pass"`
	AskVaultPass       bool          `mapstructure:"ask_vault_pass"`
	TransportMode      string        `mapstructure:"transport"`
	RemotePort         int           `mapstructure:"remote_port"`
	ModuleLang         string        `mapstructure:"module_lang"`
	ModuleSetLocale    bool          `mapstructure:"module_set_locale"`

	// Paths
	InventoryFile      string   `mapstructure:"inventory"`
	LibraryPath        []string `mapstructure:"library"`
	ModulePath         []string `mapstructure:"module_path"`
	ActionPluginPath   []string `mapstructure:"action_plugins"`
	CachePluginPath    []string `mapstructure:"cache_plugins"`
	CallbackPluginPath []string `mapstructure:"callback_plugins"`
	ConnectionPluginPath []string `mapstructure:"connection_plugins"`
	LookupPluginPath   []string `mapstructure:"lookup_plugins"`
	FilterPluginPath   []string `mapstructure:"filter_plugins"`
	TestPluginPath     []string `mapstructure:"test_plugins"`
	StrategyPluginPath []string `mapstructure:"strategy_plugins"`
	VarsPluginPath     []string `mapstructure:"vars_plugins"`
	RolesPath          []string `mapstructure:"roles_path"`
	HostsFile          string   `mapstructure:"hostfile"`

	// Output
	NoColor            bool   `mapstructure:"no_color"`
	NoLog              bool   `mapstructure:"no_log"`
	DisplaySkippedHosts bool  `mapstructure:"display_skipped_hosts"`
	DisplayOkHosts     bool   `mapstructure:"display_ok_hosts"`
	DisplayFailed      bool   `mapstructure:"display_failed_hosts"`
	ShowCustomStats    bool   `mapstructure:"show_custom_stats"`
	CallbackWhitelist  []string `mapstructure:"callback_whitelist"`
	StdoutCallback     string `mapstructure:"stdout_callback"`
	BinAnsibleCallbacks bool  `mapstructure:"bin_ansible_callbacks"`

	// Connection
	SSHArgs            string `mapstructure:"ssh_args"`
	ControlPath        string `mapstructure:"control_path"`
	ControlPathDir     string `mapstructure:"control_path_dir"`
	ControlPersist     string `mapstructure:"control_persist"`
	Pipelining         bool   `mapstructure:"pipelining"`
	SCP                string `mapstructure:"scp_if_ssh"`
	SFTP               bool   `mapstructure:"sftp_batch_mode"`

	// Privilege escalation
	Become             bool   `mapstructure:"become"`
	BecomeMethod       string `mapstructure:"become_method"`
	BecomeUser         string `mapstructure:"become_user"`
	BecomeAskPass      bool   `mapstructure:"become_ask_pass"`
	BecomeExe          string `mapstructure:"become_exe"`
	BecomeFlags        string `mapstructure:"become_flags"`

	// Playbook
	ForceHandlers      bool `mapstructure:"force_handlers"`
	FlushCache         bool `mapstructure:"flush_cache"`
	GatherFacts        string `mapstructure:"gathering"`
	GatherSubset       []string `mapstructure:"gather_subset"`
	GatherTimeout      time.Duration `mapstructure:"gather_timeout"`
	FactCaching        string `mapstructure:"fact_caching"`
	FactCachingConnection string `mapstructure:"fact_caching_connection"`
	FactCachingPrefix  string `mapstructure:"fact_caching_prefix"`
	FactCachingTimeout time.Duration `mapstructure:"fact_caching_timeout"`

	// Advanced
	HashBehaviour      string `mapstructure:"hash_behaviour"`
	HostPatternMismatch string `mapstructure:"host_pattern_mismatch"`
	JinjaExtensions    []string `mapstructure:"jinja2_extensions"`
	Retry              bool   `mapstructure:"retry_files_enabled"`
	RetryFilesSavePath string `mapstructure:"retry_files_save_path"`
	LogPath            string `mapstructure:"log_path"`
	VaultPasswordFile  string `mapstructure:"vault_password_file"`
	VaultEncryptIdentity string `mapstructure:"vault_encrypt_identity"`
	VaultIdMatch       bool   `mapstructure:"vault_id_match"`

	// Inventory
	InventoryIgnoreRegex []string `mapstructure:"inventory_ignore_extensions"`
	InventoryEnabled     []string `mapstructure:"inventory_enabled"`
	HostnamePattern      string   `mapstructure:"hostname_pattern"`

	// Galaxy
	GalaxyServerList   []string `mapstructure:"galaxy_server_list"`
	GalaxyIgnoreCerts  bool     `mapstructure:"galaxy_ignore_certs"`
	GalaxyRole         string   `mapstructure:"galaxy_role_skeleton"`
	GalaxyRoleIgnore   []string `mapstructure:"galaxy_role_skeleton_ignore"`

	fs afero.Fs
}

// Manager handles Ansible configuration management with multiple source support
type Manager struct {
	config *Config
	viper  *viper.Viper
	fs     afero.Fs
}

// NewManager creates a new configuration manager
func NewManager(fs afero.Fs) *Manager {
	v := viper.New()
	v.SetFs(fs)

	return &Manager{
		config: &Config{fs: fs},
		viper:  v,
		fs:     fs,
	}
}

// LoadConfig loads configuration from multiple sources with proper precedence
func (m *Manager) LoadConfig() error {
	// Set defaults first
	m.setDefaults()

	// Configure Viper to read from multiple config file types
	m.viper.SetConfigName("ansible")
	m.viper.SetConfigType("yaml") // Default, but we'll try others

	// Add configuration paths in order of precedence (lowest to highest)
	m.addConfigPaths()

	// Set environment variable prefix and automatic env binding
	m.viper.SetEnvPrefix("ANSIBLE")
	m.viper.AutomaticEnv()
	m.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Try to read the configuration file
	if err := m.readConfigFile(); err != nil {
		// Config file not found is not an error - we can work with defaults
		// and environment variables
		if !isConfigNotFoundError(err) {
			return fmt.Errorf("error reading config file: %w", err)
		}
		// If config file not found, that's okay - we'll use defaults
	}

	// Unmarshal into our config struct
	if err := m.viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Post-process configuration
	m.processConfig()

	return nil
}

// setDefaults sets default configuration values matching Ansible's defaults
func (m *Manager) setDefaults() {
	m.viper.SetDefault("private_key_file", "~/.ssh/id_rsa")
	m.viper.SetDefault("host_key_checking", true)
	m.viper.SetDefault("timeout", "10s")
	m.viper.SetDefault("remote_user", getCurrentUser())
	m.viper.SetDefault("ask_pass", false)
	m.viper.SetDefault("ask_sudo_pass", false)
	m.viper.SetDefault("ask_vault_pass", false)
	m.viper.SetDefault("transport", "smart")
	m.viper.SetDefault("remote_port", 22)
	m.viper.SetDefault("module_lang", "C")
	m.viper.SetDefault("module_set_locale", false)

	// Paths
	m.viper.SetDefault("inventory", "/etc/ansible/hosts")
	m.viper.SetDefault("roles_path", []string{"~/.ansible/roles", "/usr/share/ansible/roles", "/etc/ansible/roles"})
	m.viper.SetDefault("hostfile", "/etc/ansible/hosts")

	// Output
	m.viper.SetDefault("no_color", false)
	m.viper.SetDefault("no_log", false)
	m.viper.SetDefault("display_skipped_hosts", true)
	m.viper.SetDefault("display_ok_hosts", true)
	m.viper.SetDefault("display_failed_hosts", true)
	m.viper.SetDefault("show_custom_stats", false)
	m.viper.SetDefault("stdout_callback", "default")
	m.viper.SetDefault("bin_ansible_callbacks", false)

	// Connection
	m.viper.SetDefault("ssh_args", "-C -o ControlMaster=auto -o ControlPersist=60s")
	m.viper.SetDefault("control_path", "~/.ansible/cp/ansible-ssh-%%h-%%p-%%r")
	m.viper.SetDefault("control_path_dir", "~/.ansible/cp")
	m.viper.SetDefault("control_persist", "60s")
	m.viper.SetDefault("pipelining", false)
	m.viper.SetDefault("scp_if_ssh", "smart")
	m.viper.SetDefault("sftp_batch_mode", true)

	// Privilege escalation
	m.viper.SetDefault("become", false)
	m.viper.SetDefault("become_method", "sudo")
	m.viper.SetDefault("become_user", "root")
	m.viper.SetDefault("become_ask_pass", false)

	// Playbook
	m.viper.SetDefault("force_handlers", false)
	m.viper.SetDefault("flush_cache", false)
	m.viper.SetDefault("gathering", "implicit")
	m.viper.SetDefault("gather_subset", []string{"all"})
	m.viper.SetDefault("gather_timeout", "10s")
	m.viper.SetDefault("fact_caching", "memory")
	m.viper.SetDefault("fact_caching_timeout", "86400s")

	// Advanced
	m.viper.SetDefault("hash_behaviour", "replace")
	m.viper.SetDefault("host_pattern_mismatch", "warning")
	m.viper.SetDefault("retry_files_enabled", false)
	m.viper.SetDefault("retry_files_save_path", "~/.ansible-retry")
	m.viper.SetDefault("vault_id_match", false)

	// Inventory
	m.viper.SetDefault("inventory_ignore_extensions", []string{
		"~", ".orig", ".bak", ".ini", ".cfg", ".retry", ".pyc", ".pyo",
	})
	m.viper.SetDefault("inventory_enabled", []string{
		"host_list", "script", "auto", "yaml", "ini", "toml",
	})

	// Galaxy
	m.viper.SetDefault("galaxy_ignore_certs", false)
}

// addConfigPaths adds configuration file search paths in order of precedence
func (m *Manager) addConfigPaths() {
	// Current directory (highest precedence)
	m.viper.AddConfigPath(".")

	// User's home directory
	if home, err := os.UserHomeDir(); err == nil {
		m.viper.AddConfigPath(home)
		m.viper.AddConfigPath(filepath.Join(home, ".ansible"))
	}

	// System directories (lowest precedence)
	m.viper.AddConfigPath("/etc/ansible")
	m.viper.AddConfigPath("/usr/local/etc/ansible")
}

// readConfigFile attempts to read configuration files in multiple formats
func (m *Manager) readConfigFile() error {
	formats := []string{"yaml", "yml", "json", "toml", "ini"}
	var lastErr error

	for _, format := range formats {
		m.viper.SetConfigType(format)
		if err := m.viper.ReadInConfig(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("no configuration file found")
}

// processConfig performs post-processing on loaded configuration
func (m *Manager) processConfig() {
	// Expand paths
	m.config.PrivateKeyFile = expandPath(m.config.PrivateKeyFile)
	m.config.InventoryFile = expandPath(m.config.InventoryFile)
	m.config.LogPath = expandPath(m.config.LogPath)
	m.config.VaultPasswordFile = expandPath(m.config.VaultPasswordFile)
	m.config.RetryFilesSavePath = expandPath(m.config.RetryFilesSavePath)
	m.config.ControlPath = expandPath(m.config.ControlPath)
	m.config.ControlPathDir = expandPath(m.config.ControlPathDir)

	// Expand path lists
	m.config.RolesPath = expandPaths(m.config.RolesPath)
	m.config.LibraryPath = expandPaths(m.config.LibraryPath)
	m.config.ModulePath = expandPaths(m.config.ModulePath)
	m.config.ActionPluginPath = expandPaths(m.config.ActionPluginPath)
	m.config.CachePluginPath = expandPaths(m.config.CachePluginPath)
	m.config.CallbackPluginPath = expandPaths(m.config.CallbackPluginPath)
	m.config.ConnectionPluginPath = expandPaths(m.config.ConnectionPluginPath)
	m.config.LookupPluginPath = expandPaths(m.config.LookupPluginPath)
	m.config.FilterPluginPath = expandPaths(m.config.FilterPluginPath)
	m.config.TestPluginPath = expandPaths(m.config.TestPluginPath)
	m.config.StrategyPluginPath = expandPaths(m.config.StrategyPluginPath)
	m.config.VarsPluginPath = expandPaths(m.config.VarsPluginPath)
}

// GetConfig returns the loaded configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// GetValue returns a configuration value by key
func (m *Manager) GetValue(key string) interface{} {
	return m.viper.Get(key)
}

// SetValue sets a configuration value
func (m *Manager) SetValue(key string, value interface{}) {
	m.viper.Set(key, value)
}

// LoadConfigFromData loads configuration directly from data (for testing)
func (m *Manager) LoadConfigFromData(data []byte, format string) error {
	// Set defaults first
	m.setDefaults()

	// Set environment variable prefix and automatic env binding
	m.viper.SetEnvPrefix("ANSIBLE")
	m.viper.AutomaticEnv()
	m.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Set the config type and read from the data
	m.viper.SetConfigType(format)
	if err := m.viper.ReadConfig(strings.NewReader(string(data))); err != nil {
		return fmt.Errorf("error reading config from data: %w", err)
	}

	// Unmarshal into our config struct
	if err := m.viper.Unmarshal(m.config); err != nil {
		return fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Post-process configuration
	m.processConfig()

	return nil
}

// Helper functions

// getCurrentUser returns the current user name
func getCurrentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	if user := os.Getenv("USERNAME"); user != "" {
		return user
	}
	return "ansible"
}

// expandPath expands ~ and environment variables in paths
func expandPath(path string) string {
	if path == "" {
		return path
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Expand home directory
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	return path
}

// expandPaths expands multiple paths
func expandPaths(paths []string) []string {
	expanded := make([]string, len(paths))
	for i, path := range paths {
		expanded[i] = expandPath(path)
	}
	return expanded
}

// isConfigNotFoundError checks if the error is due to config file not being found
func isConfigNotFoundError(err error) bool {
	return strings.Contains(err.Error(), "Not Found") ||
		   strings.Contains(err.Error(), "no such file")
}