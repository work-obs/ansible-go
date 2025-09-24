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
	"net"
	"os"
	"os/user"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// SetupActionPlugin implements the setup action plugin for gathering system facts
type SetupActionPlugin struct {
	*BaseActionPlugin
}

// NewSetupActionPlugin creates a new setup action plugin
func NewSetupActionPlugin() *SetupActionPlugin {
	return &SetupActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"setup",
			"Gathers facts about remote hosts",
			"1.0.0",
			"Ansible Project",
		),
	}
}

// Run executes the setup action plugin
func (a *SetupActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	// Get filter pattern if specified
	filter := GetArgString(args, "filter", "*")

	// Get gather_subset if specified
	gatherSubset := GetArgStringSlice(args, "gather_subset")
	if gatherSubset == nil {
		gatherSubset = []string{"all"}
	}

	// Gather facts based on subset
	facts := make(map[string]interface{})

	// Always gather basic facts
	if a.shouldGather("all", gatherSubset) || a.shouldGather("min", gatherSubset) {
		if err := a.gatherBasicFacts(facts); err != nil {
			return &plugins.ActionResult{
				Failed:  true,
				Message: fmt.Sprintf("Failed to gather basic facts: %v", err),
			}, nil
		}
	}

	// Gather network facts
	if a.shouldGather("all", gatherSubset) || a.shouldGather("network", gatherSubset) {
		if err := a.gatherNetworkFacts(facts); err != nil {
			return &plugins.ActionResult{
				Failed:  true,
				Message: fmt.Sprintf("Failed to gather network facts: %v", err),
			}, nil
		}
	}

	// Gather hardware facts
	if a.shouldGather("all", gatherSubset) || a.shouldGather("hardware", gatherSubset) {
		if err := a.gatherHardwareFacts(facts); err != nil {
			return &plugins.ActionResult{
				Failed:  true,
				Message: fmt.Sprintf("Failed to gather hardware facts: %v", err),
			}, nil
		}
	}

	// Apply filter if specified
	if filter != "*" {
		filteredFacts := make(map[string]interface{})
		for key, value := range facts {
			if a.matchesFilter(key, filter) {
				filteredFacts[key] = value
			}
		}
		facts = filteredFacts
	}

	result := &plugins.ActionResult{
		Changed: false,
		Results: facts,
	}

	// Add ansible_facts to match Ansible behavior
	result.Results["ansible_facts"] = facts

	return result, nil
}

// shouldGather determines if a fact subset should be gathered
func (a *SetupActionPlugin) shouldGather(subset string, gatherSubset []string) bool {
	for _, s := range gatherSubset {
		if s == "all" || s == subset {
			return true
		}
		if strings.HasPrefix(s, "!") && s[1:] == subset {
			return false
		}
	}
	return false
}

// matchesFilter checks if a fact key matches the filter pattern
func (a *SetupActionPlugin) matchesFilter(key, filter string) bool {
	// Simple glob pattern matching - in real implementation would use filepath.Match
	if filter == "*" {
		return true
	}

	// Simple prefix matching for now
	if strings.HasSuffix(filter, "*") {
		prefix := strings.TrimSuffix(filter, "*")
		return strings.HasPrefix(key, prefix)
	}

	return key == filter
}

// gatherBasicFacts gathers basic system facts
func (a *SetupActionPlugin) gatherBasicFacts(facts map[string]interface{}) error {
	// Operating system facts
	facts["ansible_system"] = runtime.GOOS
	facts["ansible_architecture"] = runtime.GOARCH
	facts["ansible_machine"] = runtime.GOARCH

	// Hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	facts["ansible_hostname"] = hostname
	facts["ansible_nodename"] = hostname
	facts["ansible_fqdn"] = hostname // Simplified - real implementation would resolve FQDN

	// User facts
	currentUser, err := user.Current()
	if err == nil {
		facts["ansible_user_id"] = currentUser.Username
		facts["ansible_user_uid"] = currentUser.Uid
		facts["ansible_user_gid"] = currentUser.Gid
		facts["ansible_user_gecos"] = currentUser.Name
		facts["ansible_user_dir"] = currentUser.HomeDir
		facts["ansible_user_shell"] = "/bin/sh" // Default - could be enhanced
	}

	// Environment
	facts["ansible_env"] = getEnvironmentVars()

	// Go runtime facts
	facts["ansible_go_version"] = runtime.Version()
	facts["ansible_go_max_procs"] = runtime.GOMAXPROCS(0)
	facts["ansible_go_num_cpu"] = runtime.NumCPU()
	facts["ansible_go_num_goroutine"] = runtime.NumGoroutine()

	// Process ID
	facts["ansible_pid"] = os.Getpid()
	facts["ansible_ppid"] = os.Getppid()

	return nil
}

// gatherNetworkFacts gathers network-related facts
func (a *SetupActionPlugin) gatherNetworkFacts(facts map[string]interface{}) error {
	// Get network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		return err
	}

	interfaceFacts := make(map[string]interface{})
	allIPv4 := make([]string, 0)
	allIPv6 := make([]string, 0)

	for _, iface := range interfaces {
		ifaceFact := map[string]interface{}{
			"device": iface.Name,
			"flags":  iface.Flags.String(),
			"mtu":    iface.MTU,
		}

		if iface.HardwareAddr != nil {
			ifaceFact["macaddress"] = iface.HardwareAddr.String()
		}

		// Get IP addresses for this interface
		addrs, err := iface.Addrs()
		if err == nil {
			ipv4 := make([]string, 0)
			ipv6 := make([]string, 0)

			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok {
					ip := ipNet.IP
					if ip.To4() != nil {
						ipv4 = append(ipv4, ip.String())
						allIPv4 = append(allIPv4, ip.String())
					} else {
						ipv6 = append(ipv6, ip.String())
						allIPv6 = append(allIPv6, ip.String())
					}
				}
			}

			if len(ipv4) > 0 {
				ifaceFact["ipv4"] = map[string]interface{}{
					"address":   ipv4[0],
					"addresses": ipv4,
				}
			}
			if len(ipv6) > 0 {
				ifaceFact["ipv6"] = ipv6
			}
		}

		interfaceFacts[iface.Name] = ifaceFact
	}

	facts["ansible_interfaces"] = getInterfaceNames(interfaces)
	facts["ansible_all_ipv4_addresses"] = allIPv4
	facts["ansible_all_ipv6_addresses"] = allIPv6

	// Add per-interface facts
	for name, ifaceFact := range interfaceFacts {
		facts[fmt.Sprintf("ansible_%s", name)] = ifaceFact
	}

	// Default IP address (first non-loopback IPv4)
	for _, ip := range allIPv4 {
		if ip != "127.0.0.1" {
			facts["ansible_default_ipv4"] = map[string]interface{}{
				"address": ip,
			}
			break
		}
	}

	return nil
}

// gatherHardwareFacts gathers hardware-related facts
func (a *SetupActionPlugin) gatherHardwareFacts(facts map[string]interface{}) error {
	// Memory information (Unix-like systems)
	if runtime.GOOS != "windows" {
		if memInfo, err := getMemoryInfo(); err == nil {
			facts["ansible_memtotal_mb"] = memInfo.Total / 1024 / 1024
			facts["ansible_memfree_mb"] = memInfo.Available / 1024 / 1024
			facts["ansible_memory_mb"] = map[string]interface{}{
				"real": map[string]interface{}{
					"total": memInfo.Total / 1024 / 1024,
					"used":  (memInfo.Total - memInfo.Available) / 1024 / 1024,
					"free":  memInfo.Available / 1024 / 1024,
				},
			}
		}
	}

	// CPU information
	facts["ansible_processor_count"] = runtime.NumCPU()
	facts["ansible_processor_cores"] = runtime.NumCPU() // Simplified
	facts["ansible_processor_threads_per_core"] = 1     // Simplified

	// Load average (Unix-like systems)
	if runtime.GOOS != "windows" {
		if loadAvg, err := getLoadAverage(); err == nil {
			facts["ansible_loadavg"] = loadAvg
		}
	}

	// Uptime
	if uptime, err := getUptime(); err == nil {
		facts["ansible_uptime_seconds"] = int(uptime.Seconds())
	}

	return nil
}

// Helper functions

func getEnvironmentVars() map[string]string {
	env := make(map[string]string)
	for _, e := range os.Environ() {
		pair := strings.SplitN(e, "=", 2)
		if len(pair) == 2 {
			env[pair[0]] = pair[1]
		}
	}
	return env
}

func getInterfaceNames(interfaces []net.Interface) []string {
	names := make([]string, len(interfaces))
	for i, iface := range interfaces {
		names[i] = iface.Name
	}
	return names
}

// MemoryInfo represents memory information
type MemoryInfo struct {
	Total     uint64
	Available uint64
	Used      uint64
}

// getMemoryInfo gets memory information (Unix-specific)
func getMemoryInfo() (*MemoryInfo, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("memory info not implemented for Windows")
	}

	// This is a simplified implementation
	// Real implementation would read from /proc/meminfo on Linux
	// or use system calls on other Unix-like systems
	return &MemoryInfo{
		Total:     8 * 1024 * 1024 * 1024, // 8GB default
		Available: 4 * 1024 * 1024 * 1024, // 4GB default
		Used:      4 * 1024 * 1024 * 1024, // 4GB default
	}, nil
}

// getLoadAverage gets system load average (Unix-specific)
func getLoadAverage() (map[string]float64, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("load average not available on Windows")
	}

	// Simplified implementation
	// Real implementation would read from /proc/loadavg or use syscalls
	return map[string]float64{
		"1":  0.5,
		"5":  0.7,
		"15": 0.6,
	}, nil
}

// getUptime gets system uptime
func getUptime() (time.Duration, error) {
	if runtime.GOOS == "windows" {
		return 0, fmt.Errorf("uptime not implemented for Windows")
	}

	var sysinfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysinfo); err != nil {
		return 0, err
	}

	return time.Duration(sysinfo.Uptime) * time.Second, nil
}