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
	"testing"

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

	if manager.inventory == nil {
		t.Error("Expected inventory to be initialized")
	}
}

func TestNewInventory(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)

	if inv == nil {
		t.Fatal("Expected non-nil inventory")
	}

	if inv.Hosts == nil {
		t.Error("Expected hosts map to be initialized")
	}

	if inv.Groups == nil {
		t.Error("Expected groups map to be initialized")
	}

	// Check default groups
	if inv.AllGroup == nil {
		t.Error("Expected 'all' group to be initialized")
	}

	if inv.UngroupedGroup == nil {
		t.Error("Expected 'ungrouped' group to be initialized")
	}

	if _, exists := inv.Groups["all"]; !exists {
		t.Error("Expected 'all' group to be in groups map")
	}

	if _, exists := inv.Groups["ungrouped"]; !exists {
		t.Error("Expected 'ungrouped' group to be in groups map")
	}
}

func TestInventory_GetOrCreateHost(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)

	// Test creating new host
	host := inv.GetOrCreateHost("test-host")
	if host == nil {
		t.Fatal("Expected non-nil host")
	}

	if host.Name != "test-host" {
		t.Errorf("Expected host name 'test-host', got '%s'", host.Name)
	}

	if host.Variables == nil {
		t.Error("Expected host variables to be initialized")
	}

	// Test getting existing host
	host2 := inv.GetOrCreateHost("test-host")
	if host2 != host {
		t.Error("Expected to get the same host instance")
	}

	// Verify host is in inventory
	if _, exists := inv.Hosts["test-host"]; !exists {
		t.Error("Expected host to be in inventory hosts map")
	}
}

func TestInventory_GetOrCreateGroup(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)

	// Test creating new group
	group := inv.GetOrCreateGroup("test-group")
	if group == nil {
		t.Fatal("Expected non-nil group")
	}

	if group.Name != "test-group" {
		t.Errorf("Expected group name 'test-group', got '%s'", group.Name)
	}

	if group.Variables == nil {
		t.Error("Expected group variables to be initialized")
	}

	if group.Hosts == nil {
		t.Error("Expected group hosts to be initialized")
	}

	if group.Children == nil {
		t.Error("Expected group children to be initialized")
	}

	// Test getting existing group
	group2 := inv.GetOrCreateGroup("test-group")
	if group2 != group {
		t.Error("Expected to get the same group instance")
	}

	// Verify group is in inventory
	if _, exists := inv.Groups["test-group"]; !exists {
		t.Error("Expected group to be in inventory groups map")
	}
}

func TestGroup_AddHost(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)
	group := inv.GetOrCreateGroup("test-group")

	// Add host to group
	group.AddHost("host1")
	if len(group.Hosts) != 1 {
		t.Errorf("Expected 1 host, got %d", len(group.Hosts))
	}

	if group.Hosts[0] != "host1" {
		t.Errorf("Expected host 'host1', got '%s'", group.Hosts[0])
	}

	// Add same host again (should not duplicate)
	group.AddHost("host1")
	if len(group.Hosts) != 1 {
		t.Errorf("Expected 1 host after duplicate add, got %d", len(group.Hosts))
	}

	// Add different host
	group.AddHost("host2")
	if len(group.Hosts) != 2 {
		t.Errorf("Expected 2 hosts, got %d", len(group.Hosts))
	}
}

func TestGroup_AddChild(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)
	group := inv.GetOrCreateGroup("parent-group")

	// Add child to group
	group.AddChild("child1")
	if len(group.Children) != 1 {
		t.Errorf("Expected 1 child, got %d", len(group.Children))
	}

	if group.Children[0] != "child1" {
		t.Errorf("Expected child 'child1', got '%s'", group.Children[0])
	}

	// Add same child again (should not duplicate)
	group.AddChild("child1")
	if len(group.Children) != 1 {
		t.Errorf("Expected 1 child after duplicate add, got %d", len(group.Children))
	}

	// Add different child
	group.AddChild("child2")
	if len(group.Children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(group.Children))
	}
}

func TestManager_LoadFromINI(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	// Create test INI inventory
	iniContent := `
[webservers]
web1 ansible_host=192.168.1.10 ansible_port=22
web2 ansible_host=192.168.1.11

[dbservers]
db1 ansible_host=192.168.1.20

[webservers:vars]
http_port=80
maxRequestsPerChild=808
`

	// Write to virtual filesystem
	err := afero.WriteFile(fs, "/test-inventory", []byte(iniContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load inventory
	err = manager.LoadFromFile("/test-inventory")
	if err != nil {
		t.Fatalf("Failed to load inventory: %v", err)
	}

	inv := manager.GetInventory()

	// Verify hosts were created
	if len(inv.Hosts) != 3 {
		t.Errorf("Expected 3 hosts, got %d", len(inv.Hosts))
	}

	// Check web1 host
	web1, exists := inv.Hosts["web1"]
	if !exists {
		t.Error("Expected web1 host to exist")
	} else {
		if web1.Address != "192.168.1.10" {
			t.Errorf("Expected web1 address '192.168.1.10', got '%s'", web1.Address)
		}
		if web1.Port != 22 {
			t.Errorf("Expected web1 port 22, got %d", web1.Port)
		}
	}

	// Check groups were created
	webservers, exists := inv.Groups["webservers"]
	if !exists {
		t.Error("Expected webservers group to exist")
	} else {
		if len(webservers.Hosts) != 2 {
			t.Errorf("Expected 2 hosts in webservers group, got %d", len(webservers.Hosts))
		}

		// Check group variables
		if webservers.Variables["http_port"] != "80" {
			t.Errorf("Expected http_port=80, got %v", webservers.Variables["http_port"])
		}
	}

	// Check 'all' group was updated
	if len(inv.AllGroup.Hosts) != 3 {
		t.Errorf("Expected 3 hosts in 'all' group, got %d", len(inv.AllGroup.Hosts))
	}
}

func TestManager_LoadFromYAML(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	// Create test YAML inventory
	yamlContent := `
all:
  vars:
    ansible_user: admin
  children:
    webservers:
      hosts:
        web1:
          ansible_host: 192.168.1.10
        web2:
          ansible_host: 192.168.1.11
      vars:
        http_port: 80
    dbservers:
      hosts:
        db1:
          ansible_host: 192.168.1.20
          mysql_port: 3306
`

	// Write to virtual filesystem
	err := afero.WriteFile(fs, "/test-inventory.yml", []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Load inventory
	err = manager.LoadFromFile("/test-inventory.yml")
	if err != nil {
		t.Fatalf("Failed to load inventory: %v", err)
	}

	inv := manager.GetInventory()

	// Verify hosts were created
	if len(inv.Hosts) < 3 {
		t.Errorf("Expected at least 3 hosts, got %d", len(inv.Hosts))
	}

	// Check web1 host
	web1, exists := inv.Hosts["web1"]
	if !exists {
		t.Error("Expected web1 host to exist")
	} else {
		if address, ok := web1.Variables["ansible_host"].(string); !ok || address != "192.168.1.10" {
			t.Errorf("Expected web1 ansible_host '192.168.1.10', got %v", web1.Variables["ansible_host"])
		}
	}

	// Check groups were created
	webservers, exists := inv.Groups["webservers"]
	if !exists {
		t.Error("Expected webservers group to exist")
	} else {
		if len(webservers.Hosts) != 2 {
			t.Errorf("Expected 2 hosts in webservers group, got %d", len(webservers.Hosts))
		}

		// Check group variables
		if webservers.Variables["http_port"] != 80 {
			t.Errorf("Expected http_port=80, got %v", webservers.Variables["http_port"])
		}
	}

	// Check 'all' group variables
	if inv.AllGroup.Variables["ansible_user"] != "admin" {
		t.Errorf("Expected ansible_user=admin in all group, got %v", inv.AllGroup.Variables["ansible_user"])
	}
}

func TestManager_GetHosts(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	// Create test inventory
	inv := manager.GetInventory()
	host1 := inv.GetOrCreateHost("web1")
	host2 := inv.GetOrCreateHost("web2")
	host3 := inv.GetOrCreateHost("db1")

	group := inv.GetOrCreateGroup("webservers")
	group.AddHost("web1")
	group.AddHost("web2")

	inv.UpdateAllGroup()

	// Test getting all hosts
	hosts, err := manager.GetHosts("all")
	if err != nil {
		t.Fatalf("Failed to get all hosts: %v", err)
	}
	if len(hosts) != 3 {
		t.Errorf("Expected 3 hosts for 'all', got %d", len(hosts))
	}

	// Test getting hosts by group
	hosts, err = manager.GetHosts("webservers")
	if err != nil {
		t.Fatalf("Failed to get webservers hosts: %v", err)
	}
	if len(hosts) != 2 {
		t.Errorf("Expected 2 hosts for 'webservers', got %d", len(hosts))
	}

	// Test getting single host
	hosts, err = manager.GetHosts("web1")
	if err != nil {
		t.Fatalf("Failed to get web1 host: %v", err)
	}
	if len(hosts) != 1 {
		t.Errorf("Expected 1 host for 'web1', got %d", len(hosts))
	}
	if hosts[0] != host1 {
		t.Error("Expected to get host1 instance")
	}

	// Test getting non-existent group/host
	hosts, err = manager.GetHosts("nonexistent")
	if err != nil {
		t.Fatalf("Failed to get nonexistent hosts: %v", err)
	}
	if len(hosts) != 0 {
		t.Errorf("Expected 0 hosts for 'nonexistent', got %d", len(hosts))
	}

	// Suppress unused variable warnings
	_ = host2
	_ = host3
}

func TestInventory_GetHostVars(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)

	// Set up inventory
	host := inv.GetOrCreateHost("web1")
	host.Variables["host_var"] = "host_value"

	group := inv.GetOrCreateGroup("webservers")
	group.Variables["group_var"] = "group_value"
	group.AddHost("web1")

	inv.AllGroup.Variables["all_var"] = "all_value"

	// Get host variables
	vars := inv.GetHostVars("web1")

	// Check all variables are present with correct precedence
	if vars["all_var"] != "all_value" {
		t.Errorf("Expected all_var='all_value', got %v", vars["all_var"])
	}

	if vars["group_var"] != "group_value" {
		t.Errorf("Expected group_var='group_value', got %v", vars["group_var"])
	}

	if vars["host_var"] != "host_value" {
		t.Errorf("Expected host_var='host_value', got %v", vars["host_var"])
	}
}

func TestInventory_GetGroupVars(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)

	group := inv.GetOrCreateGroup("webservers")
	group.Variables["http_port"] = 80
	group.Variables["ssl_enabled"] = true

	vars := inv.GetGroupVars("webservers")

	if vars["http_port"] != 80 {
		t.Errorf("Expected http_port=80, got %v", vars["http_port"])
	}

	if vars["ssl_enabled"] != true {
		t.Errorf("Expected ssl_enabled=true, got %v", vars["ssl_enabled"])
	}

	// Test non-existent group
	vars = inv.GetGroupVars("nonexistent")
	if len(vars) != 0 {
		t.Errorf("Expected empty vars for nonexistent group, got %d vars", len(vars))
	}
}

func TestInventory_ListHosts(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)

	inv.GetOrCreateHost("host1")
	inv.GetOrCreateHost("host2")
	inv.GetOrCreateHost("host3")

	hosts := inv.ListHosts()
	if len(hosts) != 3 {
		t.Errorf("Expected 3 hosts, got %d", len(hosts))
	}

	// Check that all hosts are present (order doesn't matter)
	hostSet := make(map[string]bool)
	for _, host := range hosts {
		hostSet[host] = true
	}

	expectedHosts := []string{"host1", "host2", "host3"}
	for _, expected := range expectedHosts {
		if !hostSet[expected] {
			t.Errorf("Expected host '%s' in list", expected)
		}
	}
}

func TestInventory_ListGroups(t *testing.T) {
	fs := afero.NewMemMapFs()
	inv := NewInventory(fs)

	inv.GetOrCreateGroup("webservers")
	inv.GetOrCreateGroup("dbservers")

	groups := inv.ListGroups()
	// Should include default groups 'all' and 'ungrouped' plus the 2 we created
	if len(groups) < 4 {
		t.Errorf("Expected at least 4 groups, got %d", len(groups))
	}

	// Check that our groups are present
	groupSet := make(map[string]bool)
	for _, group := range groups {
		groupSet[group] = true
	}

	expectedGroups := []string{"all", "ungrouped", "webservers", "dbservers"}
	for _, expected := range expectedGroups {
		if !groupSet[expected] {
			t.Errorf("Expected group '%s' in list", expected)
		}
	}
}

func TestParseHostRange(t *testing.T) {
	fs := afero.NewMemMapFs()
	manager := NewManager(fs)

	// Test parsing host range
	err := manager.parseHostRange("web[1:3].example.com ansible_user=admin", "webservers")
	if err != nil {
		t.Fatalf("Failed to parse host range: %v", err)
	}

	inv := manager.GetInventory()

	// Check that hosts were created
	expectedHosts := []string{"web1.example.com", "web2.example.com", "web3.example.com"}
	for _, hostName := range expectedHosts {
		host, exists := inv.Hosts[hostName]
		if !exists {
			t.Errorf("Expected host '%s' to exist", hostName)
			continue
		}

		if user, ok := host.Variables["ansible_user"].(string); !ok || user != "admin" {
			t.Errorf("Expected ansible_user='admin' for host '%s', got %v", hostName, host.Variables["ansible_user"])
		}
	}

	// Check that hosts were added to group
	group, exists := inv.Groups["webservers"]
	if !exists {
		t.Fatal("Expected webservers group to exist")
	}

	if len(group.Hosts) != 3 {
		t.Errorf("Expected 3 hosts in webservers group, got %d", len(group.Hosts))
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		text    string
		pattern string
		should_match bool
	}{
		{"web1", "web*", true},
		{"web1", "db*", false},
		{"web1.example.com", "*.example.com", true},
		{"web1.example.com", "web1.*", true},
		{"database", "data*", true},
		{"database", "*base", true},
		{"database", "*tab*", true},
		{"web1", "web1", true},
		{"web2", "web1", false},
	}

	for _, test := range tests {
		matched, err := matchPattern(test.text, test.pattern)
		if err != nil {
			t.Errorf("Pattern matching failed for '%s' against '%s': %v", test.text, test.pattern, err)
			continue
		}

		if matched != test.should_match {
			t.Errorf("Pattern '%s' against '%s': expected %v, got %v", test.pattern, test.text, test.should_match, matched)
		}
	}
}