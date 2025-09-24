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

package lookup

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// LookupPlugin interface for lookup plugins
type LookupPlugin interface {
	plugins.BasePlugin
	Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error)
}

// BaseLookupPlugin provides common functionality for lookup plugins
type BaseLookupPlugin struct {
	name        string
	description string
	version     string
	author      string
}

func NewBaseLookupPlugin(name, description, version, author string) *BaseLookupPlugin {
	return &BaseLookupPlugin{
		name:        name,
		description: description,
		version:     version,
		author:      author,
	}
}

func (l *BaseLookupPlugin) Name() string {
	return l.name
}

func (l *BaseLookupPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeLookup
}

func (l *BaseLookupPlugin) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        l.name,
		Type:        plugins.PluginTypeLookup,
		Description: l.description,
		Version:     l.version,
		Author:      []string{l.author},
	}
}

// FileLookupPlugin implements file content lookup
type FileLookupPlugin struct {
	*BaseLookupPlugin
}

func NewFileLookupPlugin() *FileLookupPlugin {
	return &FileLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"file",
			"Read file contents",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (f *FileLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))

	for _, term := range terms {
		// Support template expansion
		filepath := f.expandPath(term, variables)

		content, err := os.ReadFile(filepath)
		if err != nil {
			// Check if error should be ignored
			if errorOnMissing, ok := options["errors"].(string); ok && errorOnMissing == "ignore" {
				continue
			}
			return nil, fmt.Errorf("failed to read file %s: %v", filepath, err)
		}

		// Trim trailing newline if requested
		contentStr := string(content)
		if rstrip, ok := options["rstrip"].(bool); ok && rstrip {
			contentStr = strings.TrimRight(contentStr, "\n\r")
		}

		results = append(results, contentStr)
	}

	return results, nil
}

func (f *FileLookupPlugin) expandPath(path string, variables map[string]interface{}) string {
	// Simple variable expansion - in real implementation would use template engine
	if strings.HasPrefix(path, "~/") {
		if home := os.Getenv("HOME"); home != "" {
			path = filepath.Join(home, path[2:])
		}
	}
	return path
}

// EnvLookupPlugin implements environment variable lookup
type EnvLookupPlugin struct {
	*BaseLookupPlugin
}

func NewEnvLookupPlugin() *EnvLookupPlugin {
	return &EnvLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"env",
			"Read environment variables",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (e *EnvLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))

	for _, term := range terms {
		value := os.Getenv(term)
		if value == "" {
			// Check for default value
			if defaultVal, ok := options["default"]; ok {
				value = fmt.Sprintf("%v", defaultVal)
			}
		}
		results = append(results, value)
	}

	return results, nil
}

// PipeLookupPlugin implements command pipeline lookup
type PipeLookupPlugin struct {
	*BaseLookupPlugin
}

func NewPipeLookupPlugin() *PipeLookupPlugin {
	return &PipeLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"pipe",
			"Execute shell commands and return output",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (p *PipeLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))

	for _, term := range terms {
		// In real implementation, would execute shell command safely
		// For now, return mock result
		result := fmt.Sprintf("mock output for command: %s", term)
		results = append(results, result)
	}

	return results, nil
}

// TemplateFileGlobLookupPlugin implements file globbing lookup
type FileGlobLookupPlugin struct {
	*BaseLookupPlugin
}

func NewFileGlobLookupPlugin() *FileGlobLookupPlugin {
	return &FileGlobLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"fileglob",
			"List files matching glob pattern",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (f *FileGlobLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0)

	for _, term := range terms {
		matches, err := filepath.Glob(term)
		if err != nil {
			return nil, fmt.Errorf("invalid glob pattern %s: %v", term, err)
		}

		for _, match := range matches {
			results = append(results, match)
		}
	}

	return results, nil
}

// FirstFoundLookupPlugin implements first found file lookup
type FirstFoundLookupPlugin struct {
	*BaseLookupPlugin
}

func NewFirstFoundLookupPlugin() *FirstFoundLookupPlugin {
	return &FirstFoundLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"first_found",
			"Return first file found from list",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (f *FirstFoundLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	var paths []string

	// Handle both simple terms and complex objects
	for _, term := range terms {
		paths = append(paths, term)
	}

	// Also check options for files/paths
	if files, ok := options["files"].([]interface{}); ok {
		for _, file := range files {
			paths = append(paths, fmt.Sprintf("%v", file))
		}
	}

	// Check each path
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return []interface{}{path}, nil
		}
	}

	// Check if we should skip when no files found
	if skip, ok := options["skip"].(bool); ok && skip {
		return []interface{}{}, nil
	}

	return nil, fmt.Errorf("no files found in: %s", strings.Join(paths, ", "))
}

// LinesLookupPlugin implements line-by-line file reading
type LinesLookupPlugin struct {
	*BaseLookupPlugin
}

func NewLinesLookupPlugin() *LinesLookupPlugin {
	return &LinesLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"lines",
			"Read file lines",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (l *LinesLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0)

	for _, term := range terms {
		file, err := os.Open(term)
		if err != nil {
			return nil, fmt.Errorf("failed to open file %s: %v", term, err)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			// Strip whitespace if requested
			if strip, ok := options["strip"].(bool); ok && strip {
				line = strings.TrimSpace(line)
			}
			results = append(results, line)
		}

		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("error reading file %s: %v", term, err)
		}
	}

	return results, nil
}

// URLLookupPlugin implements HTTP URL lookup
type URLLookupPlugin struct {
	*BaseLookupPlugin
}

func NewURLLookupPlugin() *URLLookupPlugin {
	return &URLLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"url",
			"Fetch content from HTTP URLs",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (u *URLLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))

	// Create HTTP client with options
	client := &http.Client{}

	// Handle TLS validation options
	if validate, ok := options["validate_certs"].(bool); ok && !validate {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	for _, term := range terms {
		req, err := http.NewRequestWithContext(ctx, "GET", term, nil)
		if err != nil {
			return nil, fmt.Errorf("invalid URL %s: %v", term, err)
		}

		// Add headers if specified
		if headers, ok := options["headers"].(map[string]interface{}); ok {
			for key, value := range headers {
				req.Header.Set(key, fmt.Sprintf("%v", value))
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch %s: %v", term, err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response from %s: %v", term, err)
		}

		results = append(results, string(body))
	}

	return results, nil
}

// PasswordLookupPlugin implements password generation/retrieval
type PasswordLookupPlugin struct {
	*BaseLookupPlugin
}

func NewPasswordLookupPlugin() *PasswordLookupPlugin {
	return &PasswordLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"password",
			"Generate or retrieve passwords",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (p *PasswordLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0, len(terms))

	for _, term := range terms {
		// Extract path and options from term
		parts := strings.Split(term, " ")
		path := parts[0]

		// Check if password file already exists
		if _, err := os.Stat(path); err == nil {
			// Read existing password
			content, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("failed to read password file %s: %v", path, err)
			}
			password := strings.TrimSpace(string(content))
			results = append(results, password)
			continue
		}

		// Generate new password
		length := 16
		if l, ok := options["length"].(int); ok {
			length = l
		}

		// Parse length from term if specified
		for _, part := range parts[1:] {
			if strings.HasPrefix(part, "length=") {
				if l, err := strconv.Atoi(strings.TrimPrefix(part, "length=")); err == nil {
					length = l
				}
			}
		}

		password := p.generatePassword(length, options)

		// Save password to file
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", dir, err)
		}

		if err := os.WriteFile(path, []byte(password), 0600); err != nil {
			return nil, fmt.Errorf("failed to save password to %s: %v", path, err)
		}

		results = append(results, password)
	}

	return results, nil
}

func (p *PasswordLookupPlugin) generatePassword(length int, options map[string]interface{}) string {
	// Simple password generation - in real implementation would be more sophisticated
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	// Add special characters if requested
	if chars_opt, ok := options["chars"].(string); ok {
		chars = chars_opt
	}

	password := make([]byte, length)
	for i := range password {
		password[i] = chars[i%len(chars)] // Simplified - should be random
	}

	return string(password)
}

// SequenceLookupPlugin implements sequence generation
type SequenceLookupPlugin struct {
	*BaseLookupPlugin
}

func NewSequenceLookupPlugin() *SequenceLookupPlugin {
	return &SequenceLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"sequence",
			"Generate sequence of numbers",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (s *SequenceLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0)

	for _, term := range terms {
		// Parse sequence specification (e.g., "start=1 end=10 stride=2")
		start, end, stride, format := s.parseSequence(term)

		for i := start; i <= end; i += stride {
			if format != "" {
				results = append(results, fmt.Sprintf(format, i))
			} else {
				results = append(results, i)
			}
		}
	}

	return results, nil
}

func (s *SequenceLookupPlugin) parseSequence(term string) (start, end, stride int, format string) {
	start = 1
	end = 1
	stride = 1
	format = ""

	parts := strings.Fields(term)
	for _, part := range parts {
		if strings.Contains(part, "=") {
			kv := strings.Split(part, "=")
			if len(kv) == 2 {
				key, value := kv[0], kv[1]
				switch key {
				case "start":
					if i, err := strconv.Atoi(value); err == nil {
						start = i
					}
				case "end":
					if i, err := strconv.Atoi(value); err == nil {
						end = i
					}
				case "stride":
					if i, err := strconv.Atoi(value); err == nil {
						stride = i
					}
				case "format":
					format = value
				}
			}
		} else {
			// Simple number range (e.g., "1-10")
			if strings.Contains(part, "-") {
				rangeParts := strings.Split(part, "-")
				if len(rangeParts) == 2 {
					if s, err := strconv.Atoi(rangeParts[0]); err == nil {
						start = s
					}
					if e, err := strconv.Atoi(rangeParts[1]); err == nil {
						end = e
					}
				}
			}
		}
	}

	return start, end, stride, format
}

// CSVFileLookupPlugin implements CSV file lookup
type CSVFileLookupPlugin struct {
	*BaseLookupPlugin
}

func NewCSVFileLookupPlugin() *CSVFileLookupPlugin {
	return &CSVFileLookupPlugin{
		BaseLookupPlugin: NewBaseLookupPlugin(
			"csvfile",
			"Read CSV file values",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (c *CSVFileLookupPlugin) Run(ctx context.Context, terms []string, variables map[string]interface{}, options map[string]interface{}) ([]interface{}, error) {
	results := make([]interface{}, 0)

	for _, term := range terms {
		// Parse term: "key file=path.csv delimiter=, col=1"
		key, filename, col, delimiter := c.parseCSVTerm(term)

		file, err := os.Open(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to open CSV file %s: %v", filename, err)
		}
		defer file.Close()

		reader := csv.NewReader(file)
		if delimiter != "" && len(delimiter) > 0 {
			reader.Comma = rune(delimiter[0])
		}

		records, err := reader.ReadAll()
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV file %s: %v", filename, err)
		}

		// Search for key in first column and return value from specified column
		for _, record := range records {
			if len(record) > 0 && record[0] == key {
				if col < len(record) {
					results = append(results, record[col])
				}
				break
			}
		}
	}

	return results, nil
}

func (c *CSVFileLookupPlugin) parseCSVTerm(term string) (key, filename string, col int, delimiter string) {
	parts := strings.Fields(term)
	key = parts[0]
	col = 1
	delimiter = ","

	for _, part := range parts[1:] {
		if strings.Contains(part, "=") {
			kv := strings.Split(part, "=")
			if len(kv) == 2 {
				switch kv[0] {
				case "file":
					filename = kv[1]
				case "col":
					if c, err := strconv.Atoi(kv[1]); err == nil {
						col = c
					}
				case "delimiter":
					delimiter = kv[1]
				}
			}
		}
	}

	return key, filename, col, delimiter
}

// LookupPluginRegistry manages lookup plugin registration and creation
type LookupPluginRegistry struct {
	plugins map[string]func() LookupPlugin
}

func NewLookupPluginRegistry() *LookupPluginRegistry {
	registry := &LookupPluginRegistry{
		plugins: make(map[string]func() LookupPlugin),
	}

	// Register built-in lookup plugins
	registry.Register("file", func() LookupPlugin { return NewFileLookupPlugin() })
	registry.Register("env", func() LookupPlugin { return NewEnvLookupPlugin() })
	registry.Register("pipe", func() LookupPlugin { return NewPipeLookupPlugin() })
	registry.Register("fileglob", func() LookupPlugin { return NewFileGlobLookupPlugin() })
	registry.Register("first_found", func() LookupPlugin { return NewFirstFoundLookupPlugin() })
	registry.Register("lines", func() LookupPlugin { return NewLinesLookupPlugin() })
	registry.Register("url", func() LookupPlugin { return NewURLLookupPlugin() })
	registry.Register("password", func() LookupPlugin { return NewPasswordLookupPlugin() })
	registry.Register("sequence", func() LookupPlugin { return NewSequenceLookupPlugin() })
	registry.Register("csvfile", func() LookupPlugin { return NewCSVFileLookupPlugin() })

	return registry
}

func (r *LookupPluginRegistry) Register(name string, creator func() LookupPlugin) {
	r.plugins[name] = creator
}

func (r *LookupPluginRegistry) Get(name string) (LookupPlugin, error) {
	creator, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("lookup plugin '%s' not found", name)
	}
	return creator(), nil
}

func (r *LookupPluginRegistry) Exists(name string) bool {
	_, exists := r.plugins[name]
	return exists
}

func (r *LookupPluginRegistry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}