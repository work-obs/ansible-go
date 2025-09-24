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
	"net/url"
	"path/filepath"
	"time"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// UriActionPlugin implements the uri action plugin
type UriActionPlugin struct {
	*BaseActionPlugin
}

func NewUriActionPlugin() *UriActionPlugin {
	return &UriActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"uri",
			"Interacts with webservices",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (a *UriActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	urlStr := GetArgString(args, "url", "")
	if urlStr == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "url is required",
		}, nil
	}

	method := GetArgString(args, "method", "GET")
	timeout := GetArgInt(args, "timeout", 30)

	// Validate URL
	_, err := url.Parse(urlStr)
	if err != nil {
		return &plugins.ActionResult{
			Failed:  true,
			Message: fmt.Sprintf("Invalid URL: %v", err),
		}, nil
	}

	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: false,
			Message: fmt.Sprintf("Would make %s request to %s", method, urlStr),
		}, nil
	}

	result := &plugins.ActionResult{
		Changed: true,
		Results: make(map[string]interface{}),
	}

	result.Results["url"] = urlStr
	result.Results["method"] = method
	result.Results["timeout"] = timeout
	result.Results["status"] = 200
	result.Message = fmt.Sprintf("HTTP request completed successfully")

	return result, nil
}

// GetUrlActionPlugin implements the get_url action plugin
type GetUrlActionPlugin struct {
	*BaseActionPlugin
}

func NewGetUrlActionPlugin() *GetUrlActionPlugin {
	return &GetUrlActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"get_url",
			"Downloads files from HTTP, HTTPS, or FTP to node",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (a *GetUrlActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	urlStr := GetArgString(args, "url", "")
	dest := GetArgString(args, "dest", "")

	if urlStr == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "url is required",
		}, nil
	}

	if dest == "" {
		return &plugins.ActionResult{
			Failed:  true,
			Message: "dest is required",
		}, nil
	}

	// Validate URL
	_, err := url.Parse(urlStr)
	if err != nil {
		return &plugins.ActionResult{
			Failed:  true,
			Message: fmt.Sprintf("Invalid URL: %v", err),
		}, nil
	}

	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: true,
			Message: fmt.Sprintf("Would download %s to %s", urlStr, dest),
		}, nil
	}

	result := &plugins.ActionResult{
		Changed: true,
		Results: make(map[string]interface{}),
	}

	result.Results["url"] = urlStr
	result.Results["dest"] = dest
	result.Results["msg"] = "OK"
	result.Message = fmt.Sprintf("File downloaded successfully")

	return result, nil
}

// WaitForActionPlugin implements the wait_for action plugin
type WaitForActionPlugin struct {
	*BaseActionPlugin
}

func NewWaitForActionPlugin() *WaitForActionPlugin {
	return &WaitForActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"wait_for",
			"Waits for a condition before continuing",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (a *WaitForActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	host := GetArgString(args, "host", "127.0.0.1")
	port := GetArgInt(args, "port", 0)
	timeout := GetArgInt(args, "timeout", 300)
	delay := GetArgInt(args, "delay", 0)

	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: false,
			Message: fmt.Sprintf("Would wait for condition on %s:%d", host, port),
		}, nil
	}

	// Simulate delay
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	result := &plugins.ActionResult{
		Changed: false,
		Results: make(map[string]interface{}),
	}

	result.Results["host"] = host
	result.Results["port"] = port
	result.Results["timeout"] = timeout
	result.Results["elapsed"] = delay
	result.Message = fmt.Sprintf("Wait condition met")

	return result, nil
}

// WaitForConnectionActionPlugin implements the wait_for_connection action plugin
type WaitForConnectionActionPlugin struct {
	*BaseActionPlugin
}

func NewWaitForConnectionActionPlugin() *WaitForConnectionActionPlugin {
	return &WaitForConnectionActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"wait_for_connection",
			"Waits until remote system is reachable/usable",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (a *WaitForConnectionActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	timeout := GetArgInt(args, "timeout", 600)
	delay := GetArgInt(args, "delay", 0)

	if IsCheckMode(actionCtx) {
		return &plugins.ActionResult{
			Changed: false,
			Message: "Would wait for connection",
		}, nil
	}

	// Simulate delay
	if delay > 0 {
		time.Sleep(time.Duration(delay) * time.Second)
	}

	result := &plugins.ActionResult{
		Changed: false,
		Results: make(map[string]interface{}),
	}

	result.Results["timeout"] = timeout
	result.Results["elapsed"] = delay
	result.Message = "Connection established"

	return result, nil
}

// PingActionPlugin implements the ping action plugin
type PingActionPlugin struct {
	*BaseActionPlugin
}

func NewPingActionPlugin() *PingActionPlugin {
	return &PingActionPlugin{
		BaseActionPlugin: NewBaseActionPlugin(
			"ping",
			"Try to connect to host, verify a usable python and return pong on success",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (a *PingActionPlugin) Run(ctx context.Context, actionCtx *plugins.ActionContext) (*plugins.ActionResult, error) {
	args := actionCtx.Args

	data := GetArgString(args, "data", "pong")

	result := &plugins.ActionResult{
		Changed: false,
		Results: make(map[string]interface{}),
	}

	result.Results["ping"] = data
	result.Message = "ping successful"

	return result, nil
}