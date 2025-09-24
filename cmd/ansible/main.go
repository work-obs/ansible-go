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

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/work-obs/ansible-go/internal/server"
	"github.com/work-obs/ansible-go/pkg/config"
	"github.com/work-obs/ansible-go/pkg/executor"
	inventoryPkg "github.com/work-obs/ansible-go/pkg/inventory"
)

const (
	version = "2.19.0-go"
)

var (
	// Global flags
	cfgFile     string
	verbose     int
	inventory   string
	limit       string
	moduleArgs  string
	extraVars   map[string]string
	forks       int
	timeout     int
	connection  string
	user        string
	become      bool
	becomeUser  string
	askPass     bool
	check       bool
	diff        bool

	// Server mode flags
	serverMode   bool
	serverHost   string
	serverPort   int
	certFile     string
	keyFile      string
	daemonMode   bool
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ansible",
	Short: "Ansible Go - Define and run a single task 'playbook' against a set of hosts",
	Long: `Ansible Go is a complete reimplementation of Ansible in Go, providing
full compatibility with existing Ansible playbooks, modules, and plugins
while offering improved performance and reliability.

This command allows you to run individual Ansible modules against a set of hosts,
similar to the original ansible ad-hoc command functionality.

Examples:
  # Run a command on all hosts
  ansible all -m command -a "uptime"

  # Install a package on web servers
  ansible webservers -m apt -a "name=nginx state=present" --become

  # Copy a file to specific hosts
  ansible db* -m copy -a "src=/etc/config dest=/tmp/config"

  # Start the Ansible Go server
  ansible --server --host localhost --port 8443 --cert server.crt --key server.key`,
	Version: version,
	RunE:    runAnsible,
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ansible.cfg)")
	rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "verbose mode (-v, -vv, -vvv, or -vvvv)")
	rootCmd.PersistentFlags().StringVarP(&inventory, "inventory", "i", "", "specify inventory host path or comma separated host list")
	rootCmd.PersistentFlags().StringVar(&limit, "limit", "", "further limit selected hosts to an additional pattern")
	rootCmd.PersistentFlags().StringToStringVarP(&extraVars, "extra-vars", "e", nil, "set additional variables as key=value")
	rootCmd.PersistentFlags().IntVarP(&forks, "forks", "f", 5, "specify number of parallel processes to use")
	rootCmd.PersistentFlags().IntVarP(&timeout, "timeout", "T", 10, "override the connection timeout in seconds")
	rootCmd.PersistentFlags().StringVarP(&connection, "connection", "c", "smart", "connection type to use")
	rootCmd.PersistentFlags().StringVarP(&user, "user", "u", "", "connect as this user")
	rootCmd.PersistentFlags().BoolVarP(&become, "become", "b", false, "run operations with become")
	rootCmd.PersistentFlags().StringVar(&becomeUser, "become-user", "", "run operations as this user")
	rootCmd.PersistentFlags().BoolVarP(&askPass, "ask-pass", "k", false, "ask for connection password")
	rootCmd.PersistentFlags().BoolVarP(&check, "check", "C", false, "don't make any changes")
	rootCmd.PersistentFlags().BoolVarP(&diff, "diff", "D", false, "when changing files, show the differences")

	// Ad-hoc execution flags
	rootCmd.Flags().StringVarP(&moduleArgs, "args", "a", "", "module arguments")
	rootCmd.Flags().StringVar(&moduleArgs, "module-args", "", "module arguments (alias for --args)")

	// Server mode flags
	rootCmd.Flags().BoolVar(&serverMode, "server", false, "start in server mode")
	rootCmd.Flags().StringVar(&serverHost, "host", "localhost", "server bind address")
	rootCmd.Flags().IntVar(&serverPort, "port", 8443, "server port")
	rootCmd.Flags().StringVar(&certFile, "cert", "", "TLS certificate file")
	rootCmd.Flags().StringVar(&keyFile, "key", "", "TLS private key file")
	rootCmd.Flags().BoolVar(&daemonMode, "daemon", false, "run as daemon")

	// Bind flags to viper
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("inventory", rootCmd.PersistentFlags().Lookup("inventory"))
	viper.BindPFlag("forks", rootCmd.PersistentFlags().Lookup("forks"))
	viper.BindPFlag("timeout", rootCmd.PersistentFlags().Lookup("timeout"))
}

// initConfig reads in config file and ENV variables
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("ansible")
		viper.SetConfigType("yaml")
		viper.AddConfigPath("/etc/ansible/")
		viper.AddConfigPath("$HOME/.ansible")
		viper.AddConfigPath(".")
	}

	viper.AutomaticEnv()
	viper.SetEnvPrefix("ANSIBLE")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	if err := viper.ReadInConfig(); err == nil && verbose > 0 {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", viper.ConfigFileUsed())
	}
}

// runAnsible is the main entry point for the ansible command
func runAnsible(cmd *cobra.Command, args []string) error {
	// Initialize configuration
	fs := afero.NewOsFs()
	configManager := config.NewManager(fs)

	if err := configManager.LoadConfig(); err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ansibleConfig := configManager.GetConfig()

	// Server mode
	if serverMode {
		return runServer(ansibleConfig)
	}

	// Ad-hoc module execution mode
	if len(args) < 2 {
		return fmt.Errorf("usage: ansible <host-pattern> -m <module> [-a <args>]")
	}

	hostPattern := args[0]

	// Find module name from flags
	moduleName, err := cmd.Flags().GetString("module-name")
	if err != nil || moduleName == "" {
		// Look for -m flag
		if moduleFlag := cmd.Flag("module-name"); moduleFlag == nil {
			return fmt.Errorf("module name is required (use -m)")
		}
	}

	// If no module specified in flags, check if it's the second argument
	if moduleName == "" && len(args) >= 2 {
		if args[1] == "-m" && len(args) >= 3 {
			moduleName = args[2]
		} else {
			return fmt.Errorf("module name is required (use -m)")
		}
	}

	return runAdHoc(hostPattern, moduleName, ansibleConfig)
}

// runServer starts the Ansible Go server
func runServer(ansibleConfig *config.Config) error {
	serverConfig := &server.Config{
		Host:         serverHost,
		Port:         serverPort,
		TLSCertFile:  certFile,
		TLSKeyFile:   keyFile,
		JWTIssuer:    "ansible-go",
		JWTAudience:  []string{"ansible-api"},
		JWTTokenTTL:  24 * time.Hour,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	srv, err := server.NewServer(serverConfig, ansibleConfig)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	// Handle graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		fmt.Println("\nShutting down server...")

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := srv.Stop(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		cancel()
	}()

	fmt.Printf("Starting Ansible Go server on %s:%d\n", serverHost, serverPort)

	if daemonMode {
		// In daemon mode, we would typically fork the process
		// For now, just run in the background
		fmt.Println("Running in daemon mode...")
	}

	err = srv.Start(certFile, keyFile)
	if err != nil && err.Error() != "http: Server closed" {
		return fmt.Errorf("server error: %w", err)
	}

	<-ctx.Done()
	return nil
}

// runAdHoc executes an ad-hoc Ansible command
func runAdHoc(hostPattern, moduleName string, config *config.Config) error {
	fmt.Printf("Running module '%s' on hosts matching '%s'\n", moduleName, hostPattern)

	// Create inventory manager
	inventoryPath := inventory
	if inventoryPath == "" {
		inventoryPath = config.InventoryFile
	}

	fs := afero.NewOsFs()
	invManager := inventoryPkg.NewManager(fs)

	// Load inventory from file
	if err := invManager.LoadFromFile(inventoryPath); err != nil {
		return fmt.Errorf("failed to load inventory: %w", err)
	}

	// Load inventory
	hosts, err := invManager.GetHosts(hostPattern)
	if err != nil {
		return fmt.Errorf("failed to get hosts: %w", err)
	}

	if len(hosts) == 0 {
		fmt.Printf("No hosts matched the pattern '%s'\n", hostPattern)
		return nil
	}

	hostNames := make([]string, len(hosts))
	for i, host := range hosts {
		hostNames[i] = host.Name
	}
	fmt.Printf("Found %d host(s): %s\n", len(hosts), strings.Join(hostNames, ", "))

	// Create task executor
	execConfig := &executor.Config{
		Forks:       forks,
		Timeout:     time.Duration(timeout) * time.Second,
		Connection:  connection,
		User:        user,
		Become:      become,
		BecomeUser:  becomeUser,
		Check:       check,
		Diff:        diff,
		Verbose:     verbose,
	}

	taskExecutor, err := executor.NewTaskExecutor(execConfig, config)
	if err != nil {
		return fmt.Errorf("failed to create task executor: %w", err)
	}

	// Parse module arguments
	moduleArguments := make(map[string]interface{})
	if moduleArgs != "" {
		// Simple key=value parsing (this should be more sophisticated)
		for _, arg := range strings.Fields(moduleArgs) {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				moduleArguments[parts[0]] = parts[1]
			}
		}
	}

	// Execute the module
	results, err := taskExecutor.ExecuteModule(context.Background(), hostNames, moduleName, moduleArguments)
	if err != nil {
		return fmt.Errorf("failed to execute module: %w", err)
	}

	// Display results
	for host, result := range results {
		fmt.Printf("\n%s | %s | %s\n", host, result.Status, result.Message)
		if result.Changed {
			fmt.Printf("changed: [%s]\n", host)
		}
		if result.Failed {
			fmt.Printf("FAILED: [%s] => %s\n", host, result.Message)
		}
		if verbose > 0 && len(result.Result) > 0 {
			fmt.Printf("Output: %v\n", result.Result)
		}
	}

	return nil
}

// discoverSubcommands looks for ansible-* executables and adds them as subcommands
func discoverSubcommands() {
	// Get the directory containing the current executable
	execPath, err := os.Executable()
	if err != nil {
		return
	}
	execDir := filepath.Dir(execPath)

	// Look for PATH directories
	pathDirs := strings.Split(os.Getenv("PATH"), string(os.PathListSeparator))
	pathDirs = append([]string{execDir}, pathDirs...)

	for _, dir := range pathDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			name := entry.Name()
			if strings.HasPrefix(name, "ansible-") && name != "ansible" {
				subcommandName := strings.TrimPrefix(name, "ansible-")

				// Create a subcommand that executes the external binary
				subCmd := &cobra.Command{
					Use:   subcommandName,
					Short: fmt.Sprintf("Run %s", name),
					Run: func(cmd *cobra.Command, args []string) {
						// Execute the external command
						fullPath := filepath.Join(dir, name)
						execCmd := exec.Command(fullPath, args...)
						execCmd.Stdin = os.Stdin
						execCmd.Stdout = os.Stdout
						execCmd.Stderr = os.Stderr
						execCmd.Run()
					},
				}

				rootCmd.AddCommand(subCmd)
			}
		}
	}
}