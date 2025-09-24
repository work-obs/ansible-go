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

package become

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// BecomePlugin interface for privilege escalation plugins
type BecomePlugin interface {
	plugins.BasePlugin
	BuildBecomeCommand(command string, options map[string]interface{}) (string, error)
	CheckPasswordPrompt(output string) bool
	GetPromptString() string
}

// BaseBecomePlugin provides common functionality for become plugins
type BaseBecomePlugin struct {
	name        string
	description string
	version     string
	author      string
}

func NewBaseBecomePlugin(name, description, version, author string) *BaseBecomePlugin {
	return &BaseBecomePlugin{
		name:        name,
		description: description,
		version:     version,
		author:      author,
	}
}

func (b *BaseBecomePlugin) Name() string {
	return b.name
}

func (b *BaseBecomePlugin) Type() plugins.PluginType {
	return plugins.PluginTypeBecome
}

func (b *BaseBecomePlugin) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        b.name,
		Type:        plugins.PluginTypeBecome,
		Description: b.description,
		Version:     b.version,
		Author:      []string{b.author},
	}
}

// SudoBecomePlugin implements sudo privilege escalation
type SudoBecomePlugin struct {
	*BaseBecomePlugin
}

func NewSudoBecomePlugin() *SudoBecomePlugin {
	return &SudoBecomePlugin{
		BaseBecomePlugin: NewBaseBecomePlugin(
			"sudo",
			"Substitute User DO",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (s *SudoBecomePlugin) BuildBecomeCommand(command string, options map[string]interface{}) (string, error) {
	sudoCmd := []string{"sudo"}

	if user, ok := options["become_user"].(string); ok && user != "" {
		sudoCmd = append(sudoCmd, "-u", user)
	}

	if flags, ok := options["become_flags"].(string); ok && flags != "" {
		sudoCmd = append(sudoCmd, strings.Fields(flags)...)
	}

	// Always use -S for stdin password and -p for custom prompt
	sudoCmd = append(sudoCmd, "-S", "-p", "[sudo via ansible, key="+s.GetPromptString()+"] password:")

	sudoCmd = append(sudoCmd, "/bin/sh", "-c", command)

	return strings.Join(sudoCmd, " "), nil
}

func (s *SudoBecomePlugin) CheckPasswordPrompt(output string) bool {
	return strings.Contains(output, "[sudo via ansible")
}

func (s *SudoBecomePlugin) GetPromptString() string {
	return "ansible-sudo-prompt"
}

// SuBecomePlugin implements su privilege escalation
type SuBecomePlugin struct {
	*BaseBecomePlugin
}

func NewSuBecomePlugin() *SuBecomePlugin {
	return &SuBecomePlugin{
		BaseBecomePlugin: NewBaseBecomePlugin(
			"su",
			"Substitute User",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (s *SuBecomePlugin) BuildBecomeCommand(command string, options map[string]interface{}) (string, error) {
	suCmd := []string{"su"}

	if user, ok := options["become_user"].(string); ok && user != "" {
		suCmd = append(suCmd, user)
	} else {
		suCmd = append(suCmd, "root")
	}

	if flags, ok := options["become_flags"].(string); ok && flags != "" {
		suCmd = append(suCmd, strings.Fields(flags)...)
	} else {
		suCmd = append(suCmd, "-c")
	}

	suCmd = append(suCmd, command)

	return strings.Join(suCmd, " "), nil
}

func (s *SuBecomePlugin) CheckPasswordPrompt(output string) bool {
	return strings.Contains(output, "Password:") || strings.Contains(output, "password:")
}

func (s *SuBecomePlugin) GetPromptString() string {
	return "ansible-su-prompt"
}

// DoBecomePlugin implements doas privilege escalation
type DoBecomePlugin struct {
	*BaseBecomePlugin
}

func NewDoBecomePlugin() *DoBecomePlugin {
	return &DoBecomePlugin{
		BaseBecomePlugin: NewBaseBecomePlugin(
			"doas",
			"Do As - OpenBSD privilege escalation",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (d *DoBecomePlugin) BuildBecomeCommand(command string, options map[string]interface{}) (string, error) {
	doasCmd := []string{"doas"}

	if user, ok := options["become_user"].(string); ok && user != "" {
		doasCmd = append(doasCmd, "-u", user)
	}

	if flags, ok := options["become_flags"].(string); ok && flags != "" {
		doasCmd = append(doasCmd, strings.Fields(flags)...)
	}

	doasCmd = append(doasCmd, "/bin/sh", "-c", command)

	return strings.Join(doasCmd, " "), nil
}

func (d *DoBecomePlugin) CheckPasswordPrompt(output string) bool {
	return strings.Contains(output, "doas (")
}

func (d *DoBecomePlugin) GetPromptString() string {
	return "ansible-doas-prompt"
}

// PbrunBecomePlugin implements pbrun privilege escalation
type PbrunBecomePlugin struct {
	*BaseBecomePlugin
}

func NewPbrunBecomePlugin() *PbrunBecomePlugin {
	return &PbrunBecomePlugin{
		BaseBecomePlugin: NewBaseBecomePlugin(
			"pbrun",
			"PowerBroker pbrun",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (p *PbrunBecomePlugin) BuildBecomeCommand(command string, options map[string]interface{}) (string, error) {
	pbrunCmd := []string{"pbrun"}

	if user, ok := options["become_user"].(string); ok && user != "" {
		pbrunCmd = append(pbrunCmd, "-u", user)
	}

	if flags, ok := options["become_flags"].(string); ok && flags != "" {
		pbrunCmd = append(pbrunCmd, strings.Fields(flags)...)
	}

	pbrunCmd = append(pbrunCmd, "/bin/sh", "-c", command)

	return strings.Join(pbrunCmd, " "), nil
}

func (p *PbrunBecomePlugin) CheckPasswordPrompt(output string) bool {
	return strings.Contains(output, "Password:")
}

func (p *PbrunBecomePlugin) GetPromptString() string {
	return "ansible-pbrun-prompt"
}

// BecomePluginRegistry manages become plugin registration and creation
type BecomePluginRegistry struct {
	plugins map[string]func() BecomePlugin
}

func NewBecomePluginRegistry() *BecomePluginRegistry {
	registry := &BecomePluginRegistry{
		plugins: make(map[string]func() BecomePlugin),
	}

	// Register built-in become plugins
	registry.Register("sudo", func() BecomePlugin { return NewSudoBecomePlugin() })
	registry.Register("su", func() BecomePlugin { return NewSuBecomePlugin() })
	registry.Register("doas", func() BecomePlugin { return NewDoBecomePlugin() })
	registry.Register("pbrun", func() BecomePlugin { return NewPbrunBecomePlugin() })

	return registry
}

func (r *BecomePluginRegistry) Register(name string, creator func() BecomePlugin) {
	r.plugins[name] = creator
}

func (r *BecomePluginRegistry) Get(name string) (BecomePlugin, error) {
	creator, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("become plugin '%s' not found", name)
	}
	return creator(), nil
}

func (r *BecomePluginRegistry) Exists(name string) bool {
	_, exists := r.plugins[name]
	return exists
}

func (r *BecomePluginRegistry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}