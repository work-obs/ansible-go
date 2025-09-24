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

package callback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// CallbackPlugin interface for callback plugins
type CallbackPlugin interface {
	plugins.BasePlugin
	V2RunnerOnOk(result *plugins.ActionResult) error
	V2RunnerOnFailed(result *plugins.ActionResult) error
	V2RunnerOnUnreachable(result *plugins.ActionResult) error
	V2RunnerOnSkipped(result *plugins.ActionResult) error
	V2PlaybookOnStart(playbook *PlaybookEvent) error
	V2PlaybookOnPlayStart(play *PlayEvent) error
	V2PlaybookOnTaskStart(task *TaskEvent) error
	V2PlaybookOnStats(stats *StatsEvent) error
}

// Event types for callback plugins
type PlaybookEvent struct {
	Playbook string                 `json:"playbook"`
	Options  map[string]interface{} `json:"options"`
}

type PlayEvent struct {
	Play    string                 `json:"play"`
	Pattern string                 `json:"pattern"`
	Options map[string]interface{} `json:"options"`
}

type TaskEvent struct {
	Task   string                 `json:"task"`
	Host   string                 `json:"host"`
	Action string                 `json:"action"`
	Args   map[string]interface{} `json:"args"`
}

type StatsEvent struct {
	Processed map[string]*HostStats `json:"processed"`
	Changed   map[string]int        `json:"changed"`
	Dark      map[string]int        `json:"dark"`
	Failures  map[string]int        `json:"failures"`
	Ok        map[string]int        `json:"ok"`
	Skipped   map[string]int        `json:"skipped"`
}

type HostStats struct {
	Changed     int `json:"changed"`
	Failures    int `json:"failures"`
	Ok          int `json:"ok"`
	Skipped     int `json:"skipped"`
	Unreachable int `json:"unreachable"`
}

// BaseCallbackPlugin provides common functionality for callback plugins
type BaseCallbackPlugin struct {
	name        string
	description string
	version     string
	author      string
}

func NewBaseCallbackPlugin(name, description, version, author string) *BaseCallbackPlugin {
	return &BaseCallbackPlugin{
		name:        name,
		description: description,
		version:     version,
		author:      author,
	}
}

func (c *BaseCallbackPlugin) Name() string {
	return c.name
}

func (c *BaseCallbackPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeCallback
}

func (c *BaseCallbackPlugin) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        c.name,
		Type:        plugins.PluginTypeCallback,
		Description: c.description,
		Version:     c.version,
		Author:      []string{c.author},
	}
}

// Default implementations that do nothing - plugins can override as needed
func (c *BaseCallbackPlugin) V2RunnerOnOk(result *plugins.ActionResult) error             { return nil }
func (c *BaseCallbackPlugin) V2RunnerOnFailed(result *plugins.ActionResult) error        { return nil }
func (c *BaseCallbackPlugin) V2RunnerOnUnreachable(result *plugins.ActionResult) error   { return nil }
func (c *BaseCallbackPlugin) V2RunnerOnSkipped(result *plugins.ActionResult) error       { return nil }
func (c *BaseCallbackPlugin) V2PlaybookOnStart(playbook *PlaybookEvent) error            { return nil }
func (c *BaseCallbackPlugin) V2PlaybookOnPlayStart(play *PlayEvent) error                { return nil }
func (c *BaseCallbackPlugin) V2PlaybookOnTaskStart(task *TaskEvent) error                { return nil }
func (c *BaseCallbackPlugin) V2PlaybookOnStats(stats *StatsEvent) error                  { return nil }

// DefaultCallbackPlugin implements the default callback plugin
type DefaultCallbackPlugin struct {
	*BaseCallbackPlugin
}

func NewDefaultCallbackPlugin() *DefaultCallbackPlugin {
	return &DefaultCallbackPlugin{
		BaseCallbackPlugin: NewBaseCallbackPlugin(
			"default",
			"Default callback plugin that prints results",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (d *DefaultCallbackPlugin) V2RunnerOnOk(result *plugins.ActionResult) error {
	fmt.Printf("ok: [%s]\n", "host")
	return nil
}

func (d *DefaultCallbackPlugin) V2RunnerOnFailed(result *plugins.ActionResult) error {
	fmt.Printf("FAILED - %s\n", result.Message)
	return nil
}

func (d *DefaultCallbackPlugin) V2RunnerOnUnreachable(result *plugins.ActionResult) error {
	fmt.Printf("UNREACHABLE - %s\n", result.Message)
	return nil
}

func (d *DefaultCallbackPlugin) V2RunnerOnSkipped(result *plugins.ActionResult) error {
	fmt.Printf("skipping: [%s]\n", "host")
	return nil
}

func (d *DefaultCallbackPlugin) V2PlaybookOnStart(playbook *PlaybookEvent) error {
	fmt.Printf("\nPLAY [%s] *********************************\n", playbook.Playbook)
	return nil
}

func (d *DefaultCallbackPlugin) V2PlaybookOnPlayStart(play *PlayEvent) error {
	fmt.Printf("\nTASK [%s] *********************************\n", play.Play)
	return nil
}

func (d *DefaultCallbackPlugin) V2PlaybookOnTaskStart(task *TaskEvent) error {
	fmt.Printf("\nTASK [%s] *********************************\n", task.Task)
	return nil
}

func (d *DefaultCallbackPlugin) V2PlaybookOnStats(stats *StatsEvent) error {
	fmt.Println("\nPLAY RECAP *********************************")
	for host, hostStats := range stats.Processed {
		fmt.Printf("%-30s : ok=%-4d changed=%-4d unreachable=%-4d failed=%-4d skipped=%-4d\n",
			host, hostStats.Ok, hostStats.Changed, hostStats.Unreachable, hostStats.Failures, hostStats.Skipped)
	}
	return nil
}

// MinimalCallbackPlugin implements a minimal callback plugin
type MinimalCallbackPlugin struct {
	*BaseCallbackPlugin
}

func NewMinimalCallbackPlugin() *MinimalCallbackPlugin {
	return &MinimalCallbackPlugin{
		BaseCallbackPlugin: NewBaseCallbackPlugin(
			"minimal",
			"Minimal callback plugin",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (m *MinimalCallbackPlugin) V2RunnerOnOk(result *plugins.ActionResult) error {
	if result.Changed {
		fmt.Printf(".")
	} else {
		fmt.Printf(".")
	}
	return nil
}

func (m *MinimalCallbackPlugin) V2RunnerOnFailed(result *plugins.ActionResult) error {
	fmt.Printf("F")
	return nil
}

func (m *MinimalCallbackPlugin) V2RunnerOnUnreachable(result *plugins.ActionResult) error {
	fmt.Printf("U")
	return nil
}

func (m *MinimalCallbackPlugin) V2RunnerOnSkipped(result *plugins.ActionResult) error {
	fmt.Printf("s")
	return nil
}

// JsonCallbackPlugin implements JSON output callback plugin
type JsonCallbackPlugin struct {
	*BaseCallbackPlugin
	results []map[string]interface{}
}

func NewJsonCallbackPlugin() *JsonCallbackPlugin {
	return &JsonCallbackPlugin{
		BaseCallbackPlugin: NewBaseCallbackPlugin(
			"json",
			"JSON callback plugin",
			"1.0.0",
			"Ansible Project",
		),
		results: make([]map[string]interface{}, 0),
	}
}

func (j *JsonCallbackPlugin) V2RunnerOnOk(result *plugins.ActionResult) error {
	j.addResult("ok", result)
	return nil
}

func (j *JsonCallbackPlugin) V2RunnerOnFailed(result *plugins.ActionResult) error {
	j.addResult("failed", result)
	return nil
}

func (j *JsonCallbackPlugin) V2RunnerOnUnreachable(result *plugins.ActionResult) error {
	j.addResult("unreachable", result)
	return nil
}

func (j *JsonCallbackPlugin) V2RunnerOnSkipped(result *plugins.ActionResult) error {
	j.addResult("skipped", result)
	return nil
}

func (j *JsonCallbackPlugin) addResult(status string, result *plugins.ActionResult) {
	entry := map[string]interface{}{
		"status": status,
		"result": result,
		"time":   time.Now().Format(time.RFC3339),
	}
	j.results = append(j.results, entry)
}

func (j *JsonCallbackPlugin) V2PlaybookOnStats(stats *StatsEvent) error {
	output := map[string]interface{}{
		"stats":   stats,
		"results": j.results,
	}

	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonData))
	return nil
}

// JunitCallbackPlugin implements JUnit XML output callback plugin
type JunitCallbackPlugin struct {
	*BaseCallbackPlugin
	testCases []JunitTestCase
}

type JunitTestCase struct {
	Name      string  `xml:"name,attr"`
	ClassName string  `xml:"classname,attr"`
	Time      float64 `xml:"time,attr"`
	Failure   *string `xml:"failure,omitempty"`
	Skipped   *string `xml:"skipped,omitempty"`
}

func NewJunitCallbackPlugin() *JunitCallbackPlugin {
	return &JunitCallbackPlugin{
		BaseCallbackPlugin: NewBaseCallbackPlugin(
			"junit",
			"JUnit XML callback plugin",
			"1.0.0",
			"Ansible Project",
		),
		testCases: make([]JunitTestCase, 0),
	}
}

func (j *JunitCallbackPlugin) V2RunnerOnOk(result *plugins.ActionResult) error {
	testCase := JunitTestCase{
		Name:      "task",
		ClassName: "ansible",
		Time:      0.0,
	}
	j.testCases = append(j.testCases, testCase)
	return nil
}

func (j *JunitCallbackPlugin) V2RunnerOnFailed(result *plugins.ActionResult) error {
	failure := result.Message
	testCase := JunitTestCase{
		Name:      "task",
		ClassName: "ansible",
		Time:      0.0,
		Failure:   &failure,
	}
	j.testCases = append(j.testCases, testCase)
	return nil
}

func (j *JunitCallbackPlugin) V2RunnerOnSkipped(result *plugins.ActionResult) error {
	skipped := result.Message
	testCase := JunitTestCase{
		Name:      "task",
		ClassName: "ansible",
		Time:      0.0,
		Skipped:   &skipped,
	}
	j.testCases = append(j.testCases, testCase)
	return nil
}

// CallbackPluginRegistry manages callback plugin registration and creation
type CallbackPluginRegistry struct {
	plugins map[string]func() CallbackPlugin
}

func NewCallbackPluginRegistry() *CallbackPluginRegistry {
	registry := &CallbackPluginRegistry{
		plugins: make(map[string]func() CallbackPlugin),
	}

	// Register built-in callback plugins
	registry.Register("default", func() CallbackPlugin { return NewDefaultCallbackPlugin() })
	registry.Register("minimal", func() CallbackPlugin { return NewMinimalCallbackPlugin() })
	registry.Register("json", func() CallbackPlugin { return NewJsonCallbackPlugin() })
	registry.Register("junit", func() CallbackPlugin { return NewJunitCallbackPlugin() })

	return registry
}

func (r *CallbackPluginRegistry) Register(name string, creator func() CallbackPlugin) {
	r.plugins[name] = creator
}

func (r *CallbackPluginRegistry) Get(name string) (CallbackPlugin, error) {
	creator, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("callback plugin '%s' not found", name)
	}
	return creator(), nil
}

func (r *CallbackPluginRegistry) Exists(name string) bool {
	_, exists := r.plugins[name]
	return exists
}

func (r *CallbackPluginRegistry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}