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

package inventory

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

// Host represents a single host in the inventory
type Host struct {
	Name      string                 `json:"name" yaml:"name"`
	Address   string                 `json:"ansible_host,omitempty" yaml:"ansible_host,omitempty"`
	Port      int                    `json:"ansible_port,omitempty" yaml:"ansible_port,omitempty"`
	User      string                 `json:"ansible_user,omitempty" yaml:"ansible_user,omitempty"`
	Variables map[string]interface{} `json:"vars,omitempty" yaml:"vars,omitempty"`
	Groups    []string               `json:"groups,omitempty" yaml:"groups,omitempty"`
}

// Group represents a group of hosts in the inventory
type Group struct {
	Name      string                 `json:"name" yaml:"name"`
	Hosts     []string               `json:"hosts,omitempty" yaml:"hosts,omitempty"`
	Children  []string               `json:"children,omitempty" yaml:"children,omitempty"`
	Variables map[string]interface{} `json:"vars,omitempty" yaml:"vars,omitempty"`
}

// Inventory manages the complete inventory of hosts and groups
type Inventory struct {
	Hosts       map[string]*Host  `json:"hosts" yaml:"hosts"`
	Groups      map[string]*Group `json:"groups" yaml:"groups"`
	fs          afero.Fs
	AllGroup    *Group            `json:"-" yaml:"-"`
	UngroupedGroup *Group         `json:"-" yaml:"-"`
}

// Manager handles inventory loading and management
type Manager struct {
	inventory *Inventory
	fs        afero.Fs
	sources   []string
}

// NewManager creates a new inventory manager
func NewManager(fs afero.Fs) *Manager {
	return &Manager{
		inventory: NewInventory(fs),
		fs:        fs,
		sources:   make([]string, 0),
	}
}

// NewInventory creates a new empty inventory
func NewInventory(fs afero.Fs) *Inventory {
	inv := &Inventory{
		Hosts:  make(map[string]*Host),
		Groups: make(map[string]*Group),
		fs:     fs,
	}

	// Create default groups
	inv.AllGroup = &Group{
		Name:      "all",
		Hosts:     make([]string, 0),
		Children:  make([]string, 0),
		Variables: make(map[string]interface{}),
	}
	inv.Groups["all"] = inv.AllGroup

	inv.UngroupedGroup = &Group{
		Name:      "ungrouped",
		Hosts:     make([]string, 0),
		Variables: make(map[string]interface{}),
	}
	inv.Groups["ungrouped"] = inv.UngroupedGroup

	return inv
}

// LoadFromFile loads inventory from a file
func (m *Manager) LoadFromFile(filename string) error {
	m.sources = append(m.sources, filename)

	data, err := afero.ReadFile(m.fs, filename)
	if err != nil {
		return fmt.Errorf("failed to read inventory file %s: %w", filename, err)
	}

	// Determine file format based on extension
	if strings.HasSuffix(filename, ".yml") || strings.HasSuffix(filename, ".yaml") {
		return m.loadFromYAML(data)
	} else if strings.HasSuffix(filename, ".json") {
		return m.loadFromJSON(data)
	} else {
		// Default to INI format
		return m.loadFromINI(data)
	}
}

// LoadFromString loads inventory from a string
func (m *Manager) LoadFromString(data, format string) error {
	switch strings.ToLower(format) {
	case "yaml", "yml":
		return m.loadFromYAML([]byte(data))
	case "json":
		return m.loadFromJSON([]byte(data))
	case "ini":
		return m.loadFromINI([]byte(data))
	default:
		return fmt.Errorf("unsupported inventory format: %s", format)
	}
}

// loadFromYAML loads inventory from YAML format
func (m *Manager) loadFromYAML(data []byte) error {
	var yamlInventory map[string]interface{}
	if err := yaml.Unmarshal(data, &yamlInventory); err != nil {
		return fmt.Errorf("failed to parse YAML inventory: %w", err)
	}

	return m.parseYAMLInventory(yamlInventory)
}

// loadFromJSON loads inventory from JSON format
func (m *Manager) loadFromJSON(data []byte) error {
	// JSON inventory parsing would be implemented here
	// For now, return an error indicating it's not yet implemented
	return fmt.Errorf("JSON inventory format not yet implemented")
}

// loadFromINI loads inventory from INI format
func (m *Manager) loadFromINI(data []byte) error {
	lines := strings.Split(string(data), "\n")
	currentGroup := ""
	inVarsSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for group headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			groupName := line[1 : len(line)-1]

			// Handle group variables sections
			if strings.HasSuffix(groupName, ":vars") {
				currentGroup = strings.TrimSuffix(groupName, ":vars")
				inVarsSection = true
				continue
			}

			// Handle group children sections
			if strings.HasSuffix(groupName, ":children") {
				currentGroup = strings.TrimSuffix(groupName, ":children")
				inVarsSection = false
				// TODO: Handle children parsing
				continue
			}

			// Regular group
			currentGroup = groupName
			inVarsSection = false
			m.inventory.GetOrCreateGroup(currentGroup)
			continue
		}

		if inVarsSection {
			// Parse group variables
			if err := m.parseVariable(line, currentGroup); err != nil {
				return fmt.Errorf("failed to parse group variable: %w", err)
			}
		} else {
			// Parse host entry
			if err := m.parseHostEntry(line, currentGroup); err != nil {
				return fmt.Errorf("failed to parse host entry: %w", err)
			}
		}
	}

	m.inventory.UpdateAllGroup()
	return nil
}

// parseYAMLInventory parses YAML inventory structure
func (m *Manager) parseYAMLInventory(yamlInventory map[string]interface{}) error {
	// First pass: handle the 'all' group specially and process other groups
	for groupName, groupData := range yamlInventory {
		if groupName == "all" {
			// Handle special 'all' group
			if allData, ok := groupData.(map[string]interface{}); ok {
				if vars, ok := allData["vars"].(map[string]interface{}); ok {
					m.inventory.AllGroup.Variables = vars
				}
				if children, ok := allData["children"].(map[string]interface{}); ok {
					// Process children groups
					for childName, childData := range children {
						m.inventory.AllGroup.Children = append(m.inventory.AllGroup.Children, childName)
						// Parse the child group data
						m.parseGroupData(childName, childData)
					}
				}
			}
			continue
		}

		// Parse regular groups
		m.parseGroupData(groupName, groupData)
	}

	m.inventory.UpdateAllGroup()
	return nil
}

// parseGroupData parses a single group's data
func (m *Manager) parseGroupData(groupName string, groupData interface{}) {
	group := m.inventory.GetOrCreateGroup(groupName)
	groupMap, ok := groupData.(map[string]interface{})
	if !ok {
		return
	}

	// Parse hosts
	if hosts, ok := groupMap["hosts"].(map[string]interface{}); ok {
		for hostName, hostData := range hosts {
			host := m.inventory.GetOrCreateHost(hostName)
			if hostVars, ok := hostData.(map[string]interface{}); ok {
				for varName, varValue := range hostVars {
					if host.Variables == nil {
						host.Variables = make(map[string]interface{})
					}
					host.Variables[varName] = varValue
				}
			}
			group.AddHost(hostName)
		}
	}

	// Parse group variables
	if vars, ok := groupMap["vars"].(map[string]interface{}); ok {
		group.Variables = vars
	}

	// Parse children
	if children, ok := groupMap["children"].(map[string]interface{}); ok {
		for childName, childData := range children {
			group.AddChild(childName)
			// Recursively parse child groups
			m.parseGroupData(childName, childData)
		}
	}
}

// parseHostEntry parses a single host entry from INI format
func (m *Manager) parseHostEntry(line, groupName string) error {
	// Handle host ranges like web[1:3].example.com
	if strings.Contains(line, "[") && strings.Contains(line, ":") && strings.Contains(line, "]") {
		return m.parseHostRange(line, groupName)
	}

	// Parse single host entry
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	hostName := parts[0]
	host := m.inventory.GetOrCreateHost(hostName)

	// Parse host variables
	for _, part := range parts[1:] {
		if strings.Contains(part, "=") {
			keyValue := strings.SplitN(part, "=", 2)
			if len(keyValue) == 2 {
				if host.Variables == nil {
					host.Variables = make(map[string]interface{})
				}
				host.Variables[keyValue[0]] = keyValue[1]

				// Handle special variables
				switch keyValue[0] {
				case "ansible_host":
					host.Address = keyValue[1]
				case "ansible_port":
					if port, err := strconv.Atoi(keyValue[1]); err == nil {
						host.Port = port
					}
				case "ansible_user":
					host.User = keyValue[1]
				}
			}
		}
	}

	// Add host to group
	if groupName != "" {
		group := m.inventory.GetOrCreateGroup(groupName)
		group.AddHost(hostName)
	}

	return nil
}

// parseHostRange parses host ranges like web[1:3].example.com
func (m *Manager) parseHostRange(line, groupName string) error {
	// Regular expression to match ranges like web[1:3].example.com
	rangeRegex := regexp.MustCompile(`^([^[]*)\[(\d+):(\d+)\](.*)$`)
	matches := rangeRegex.FindStringSubmatch(strings.Fields(line)[0])

	if len(matches) != 5 {
		return fmt.Errorf("invalid host range format: %s", line)
	}

	prefix := matches[1]
	startStr := matches[2]
	endStr := matches[3]
	suffix := matches[4]

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return fmt.Errorf("invalid range start: %s", startStr)
	}

	end, err := strconv.Atoi(endStr)
	if err != nil {
		return fmt.Errorf("invalid range end: %s", endStr)
	}

	// Parse additional variables from the line
	hostVars := make(map[string]interface{})
	parts := strings.Fields(line)
	for _, part := range parts[1:] {
		if strings.Contains(part, "=") {
			keyValue := strings.SplitN(part, "=", 2)
			if len(keyValue) == 2 {
				hostVars[keyValue[0]] = keyValue[1]
			}
		}
	}

	// Create hosts for the range
	for i := start; i <= end; i++ {
		hostName := fmt.Sprintf("%s%d%s", prefix, i, suffix)
		host := m.inventory.GetOrCreateHost(hostName)

		// Copy variables to each host
		if host.Variables == nil {
			host.Variables = make(map[string]interface{})
		}
		for k, v := range hostVars {
			host.Variables[k] = v
		}

		// Handle special variables
		if address, ok := hostVars["ansible_host"].(string); ok {
			host.Address = address
		}
		if portStr, ok := hostVars["ansible_port"].(string); ok {
			if port, err := strconv.Atoi(portStr); err == nil {
				host.Port = port
			}
		}
		if user, ok := hostVars["ansible_user"].(string); ok {
			host.User = user
		}

		// Add to group
		if groupName != "" {
			group := m.inventory.GetOrCreateGroup(groupName)
			group.AddHost(hostName)
		}
	}

	return nil
}

// parseVariable parses a variable assignment
func (m *Manager) parseVariable(line, groupName string) error {
	if !strings.Contains(line, "=") {
		return nil
	}

	keyValue := strings.SplitN(line, "=", 2)
	if len(keyValue) != 2 {
		return nil
	}

	key := strings.TrimSpace(keyValue[0])
	value := strings.TrimSpace(keyValue[1])

	// Remove quotes if present
	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
		value = value[1 : len(value)-1]
	}

	group := m.inventory.GetOrCreateGroup(groupName)
	if group.Variables == nil {
		group.Variables = make(map[string]interface{})
	}
	group.Variables[key] = value

	return nil
}

// GetInventory returns the managed inventory
func (m *Manager) GetInventory() *Inventory {
	return m.inventory
}

// GetHosts returns all hosts matching the given pattern
func (m *Manager) GetHosts(pattern string) ([]*Host, error) {
	if pattern == "all" {
		var hosts []*Host
		for _, host := range m.inventory.Hosts {
			hosts = append(hosts, host)
		}
		return hosts, nil
	}

	// Check if pattern is a group name
	if group, exists := m.inventory.Groups[pattern]; exists {
		return m.getHostsFromGroup(group), nil
	}

	// Check if pattern is a host name
	if host, exists := m.inventory.Hosts[pattern]; exists {
		return []*Host{host}, nil
	}

	// Handle pattern matching (simplified)
	return m.getHostsByPattern(pattern)
}

// getHostsFromGroup returns all hosts from a group (including children)
func (m *Manager) getHostsFromGroup(group *Group) []*Host {
	hostsMap := make(map[string]*Host)

	// Add direct hosts
	for _, hostName := range group.Hosts {
		if host, exists := m.inventory.Hosts[hostName]; exists {
			hostsMap[hostName] = host
		}
	}

	// Add hosts from child groups
	for _, childName := range group.Children {
		if childGroup, exists := m.inventory.Groups[childName]; exists {
			childHosts := m.getHostsFromGroup(childGroup)
			for _, host := range childHosts {
				hostsMap[host.Name] = host
			}
		}
	}

	// Convert map to slice
	var hosts []*Host
	for _, host := range hostsMap {
		hosts = append(hosts, host)
	}

	return hosts
}

// getHostsByPattern returns hosts matching a pattern
func (m *Manager) getHostsByPattern(pattern string) ([]*Host, error) {
	// Simplified pattern matching - could be extended with regex support
	var hosts []*Host
	for _, host := range m.inventory.Hosts {
		// Simple wildcard matching
		if matched, err := matchPattern(host.Name, pattern); err == nil && matched {
			hosts = append(hosts, host)
		}
	}
	return hosts, nil
}

// GetOrCreateHost gets an existing host or creates a new one
func (inv *Inventory) GetOrCreateHost(name string) *Host {
	if host, exists := inv.Hosts[name]; exists {
		return host
	}

	host := &Host{
		Name:      name,
		Variables: make(map[string]interface{}),
		Groups:    make([]string, 0),
	}

	// Try to resolve hostname to IP
	if name != "localhost" {
		if ips, err := net.LookupIP(name); err == nil && len(ips) > 0 {
			host.Address = ips[0].String()
		}
	}

	inv.Hosts[name] = host
	return host
}

// GetOrCreateGroup gets an existing group or creates a new one
func (inv *Inventory) GetOrCreateGroup(name string) *Group {
	if group, exists := inv.Groups[name]; exists {
		return group
	}

	group := &Group{
		Name:      name,
		Hosts:     make([]string, 0),
		Children:  make([]string, 0),
		Variables: make(map[string]interface{}),
	}

	inv.Groups[name] = group
	return group
}

// AddHost adds a host to the group
func (g *Group) AddHost(hostName string) {
	// Check if host already exists in group
	for _, existing := range g.Hosts {
		if existing == hostName {
			return
		}
	}
	g.Hosts = append(g.Hosts, hostName)
}

// AddChild adds a child group to the group
func (g *Group) AddChild(childName string) {
	// Check if child already exists
	for _, existing := range g.Children {
		if existing == childName {
			return
		}
	}
	g.Children = append(g.Children, childName)
}

// UpdateAllGroup updates the 'all' group to include all hosts
func (inv *Inventory) UpdateAllGroup() {
	inv.AllGroup.Hosts = make([]string, 0)

	// Add all hosts to the 'all' group
	for hostName := range inv.Hosts {
		inv.AllGroup.Hosts = append(inv.AllGroup.Hosts, hostName)
	}

	// Update ungrouped group
	inv.updateUngroupedGroup()
}

// updateUngroupedGroup updates the 'ungrouped' group with hosts not in any other group
func (inv *Inventory) updateUngroupedGroup() {
	groupedHosts := make(map[string]bool)

	// Find all hosts that are in groups (excluding 'all' and 'ungrouped')
	for groupName, group := range inv.Groups {
		if groupName == "all" || groupName == "ungrouped" {
			continue
		}
		for _, hostName := range group.Hosts {
			groupedHosts[hostName] = true
		}
	}

	// Add ungrouped hosts
	inv.UngroupedGroup.Hosts = make([]string, 0)
	for hostName := range inv.Hosts {
		if !groupedHosts[hostName] {
			inv.UngroupedGroup.Hosts = append(inv.UngroupedGroup.Hosts, hostName)
		}
	}
}

// GetHostVars returns all variables for a host (including group variables)
func (inv *Inventory) GetHostVars(hostName string) map[string]interface{} {
	vars := make(map[string]interface{})

	// Start with 'all' group variables
	for k, v := range inv.AllGroup.Variables {
		vars[k] = v
	}

	// Add group variables for groups containing this host
	for _, group := range inv.Groups {
		for _, groupHostName := range group.Hosts {
			if groupHostName == hostName {
				for k, v := range group.Variables {
					vars[k] = v
				}
				break
			}
		}
	}

	// Add host-specific variables (highest precedence)
	if host, exists := inv.Hosts[hostName]; exists {
		for k, v := range host.Variables {
			vars[k] = v
		}
	}

	return vars
}

// GetGroupVars returns all variables for a group
func (inv *Inventory) GetGroupVars(groupName string) map[string]interface{} {
	if group, exists := inv.Groups[groupName]; exists {
		// Return a copy to prevent modification
		vars := make(map[string]interface{})
		for k, v := range group.Variables {
			vars[k] = v
		}
		return vars
	}
	return make(map[string]interface{})
}

// ListHosts returns a list of all host names
func (inv *Inventory) ListHosts() []string {
	var hosts []string
	for hostName := range inv.Hosts {
		hosts = append(hosts, hostName)
	}
	return hosts
}

// ListGroups returns a list of all group names
func (inv *Inventory) ListGroups() []string {
	var groups []string
	for groupName := range inv.Groups {
		groups = append(groups, groupName)
	}
	return groups
}

// matchPattern performs simple wildcard pattern matching
func matchPattern(text, pattern string) (bool, error) {
	// Convert glob pattern to regex
	regexPattern := strings.ReplaceAll(pattern, "*", ".*")
	regexPattern = "^" + regexPattern + "$"

	matched, err := regexp.MatchString(regexPattern, text)
	return matched, err
}

// LoadFromDirectory loads inventory from multiple files in a directory
func (m *Manager) LoadFromDirectory(dirPath string) error {
	// Check if directory exists
	exists, err := afero.DirExists(m.fs, dirPath)
	if err != nil {
		return fmt.Errorf("failed to check directory %s: %w", dirPath, err)
	}
	if !exists {
		return fmt.Errorf("directory %s does not exist", dirPath)
	}

	// Read directory contents
	files, err := afero.ReadDir(m.fs, dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	// Load each inventory file
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := file.Name()
		// Skip backup and temporary files
		if strings.HasSuffix(fileName, "~") ||
		   strings.HasSuffix(fileName, ".bak") ||
		   strings.HasPrefix(fileName, ".") {
			continue
		}

		filePath := dirPath + string(os.PathSeparator) + fileName
		if err := m.LoadFromFile(filePath); err != nil {
			return fmt.Errorf("failed to load inventory file %s: %w", filePath, err)
		}
	}

	return nil
}