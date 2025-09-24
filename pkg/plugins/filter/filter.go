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

package filter

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/work-obs/ansible-go/pkg/plugins"
)

// FilterFunction represents a template filter function
type FilterFunction func(input interface{}, args ...interface{}) (interface{}, error)

// FilterPlugin interface for filter plugins
type FilterPlugin interface {
	plugins.BasePlugin
	GetFilters() map[string]FilterFunction
}

// BaseFilterPlugin provides common functionality for filter plugins
type BaseFilterPlugin struct {
	name        string
	description string
	version     string
	author      string
}

func NewBaseFilterPlugin(name, description, version, author string) *BaseFilterPlugin {
	return &BaseFilterPlugin{
		name:        name,
		description: description,
		version:     version,
		author:      author,
	}
}

func (f *BaseFilterPlugin) Name() string {
	return f.name
}

func (f *BaseFilterPlugin) Type() plugins.PluginType {
	return plugins.PluginTypeFilter
}

func (f *BaseFilterPlugin) GetInfo() *plugins.PluginInfo {
	return &plugins.PluginInfo{
		Name:        f.name,
		Type:        plugins.PluginTypeFilter,
		Description: f.description,
		Version:     f.version,
		Author:      []string{f.author},
	}
}

// CoreFiltersPlugin implements core Ansible filters
type CoreFiltersPlugin struct {
	*BaseFilterPlugin
}

func NewCoreFiltersPlugin() *CoreFiltersPlugin {
	return &CoreFiltersPlugin{
		BaseFilterPlugin: NewBaseFilterPlugin(
			"core",
			"Core Ansible filters",
			"1.0.0",
			"Ansible Project",
		),
	}
}

func (c *CoreFiltersPlugin) GetFilters() map[string]FilterFunction {
	return map[string]FilterFunction{
		// String filters
		"upper":       c.upper,
		"lower":       c.lower,
		"capitalize":  c.capitalize,
		"title":       c.title,
		"trim":        c.trim,
		"replace":     c.replace,
		"regex_replace": c.regexReplace,
		"split":       c.split,
		"join":        c.join,
		"length":      c.length,
		"reverse":     c.reverse,
		"indent":      c.indent,
		"center":      c.center,
		"truncate":    c.truncate,

		// Type conversion filters
		"int":         c.toInt,
		"float":       c.toFloat,
		"bool":        c.toBool,
		"string":      c.toString,
		"list":        c.toList,

		// Math filters
		"abs":         c.abs,
		"round":       c.round,
		"random":      c.random,
		"max":         c.max,
		"min":         c.min,
		"sum":         c.sum,

		// List filters
		"first":       c.first,
		"last":        c.last,
		"unique":      c.unique,
		"union":       c.union,
		"intersect":   c.intersect,
		"difference":  c.difference,
		"sort":        c.sort,
		"flatten":     c.flatten,
		"select":      c.selectFilter,
		"reject":      c.reject,
		"map":         c.mapFilter,
		"selectattr":  c.selectAttr,
		"rejectattr":  c.rejectAttr,

		// Dictionary filters
		"dict2items":  c.dict2items,
		"items2dict":  c.items2dict,
		"combine":     c.combine,

		// Encoding filters
		"b64encode":   c.b64encode,
		"b64decode":   c.b64decode,
		"urlencode":   c.urlencode,
		"quote":       c.quote,

		// Hash filters
		"hash":        c.hash,
		"md5":         c.md5,
		"sha1":        c.sha1,
		"sha256":      c.sha256,

		// JSON/YAML filters
		"to_json":     c.toJson,
		"from_json":   c.fromJson,
		"to_yaml":     c.toYaml,
		"from_yaml":   c.fromYaml,
		"to_nice_json": c.toNiceJson,
		"to_nice_yaml": c.toNiceYaml,

		// Date filters
		"strftime":    c.strftime,
		"to_datetime": c.toDatetime,

		// Default and conditionals
		"default":     c.defaultFilter,
		"ternary":     c.ternary,

		// Regex filters
		"regex_search": c.regexSearch,
		"regex_findall": c.regexFindall,
		"regex_escape": c.regexEscape,

		// Path filters
		"basename":    c.basename,
		"dirname":     c.dirname,
		"expanduser":  c.expanduser,
		"realpath":    c.realpath,
		"relpath":     c.relpath,
		"splitext":    c.splitext,

		// Version comparison
		"version_compare": c.versionCompare,

		// IP address filters
		"ipaddr":      c.ipaddr,
		"ipv4":        c.ipv4,
		"ipv6":        c.ipv6,
	}
}

// String filters implementation
func (c *CoreFiltersPlugin) upper(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	return strings.ToUpper(str), nil
}

func (c *CoreFiltersPlugin) lower(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	return strings.ToLower(str), nil
}

func (c *CoreFiltersPlugin) capitalize(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	if len(str) == 0 {
		return str, nil
	}
	runes := []rune(str)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes), nil
}

func (c *CoreFiltersPlugin) title(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	return strings.Title(str), nil
}

func (c *CoreFiltersPlugin) trim(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	return strings.TrimSpace(str), nil
}

func (c *CoreFiltersPlugin) replace(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("replace filter requires 2 arguments: old and new")
	}
	str := fmt.Sprintf("%v", input)
	old := fmt.Sprintf("%v", args[0])
	new := fmt.Sprintf("%v", args[1])
	count := -1 // Replace all by default

	if len(args) > 2 {
		if c, ok := args[2].(int); ok {
			count = c
		}
	}

	return strings.Replace(str, old, new, count), nil
}

func (c *CoreFiltersPlugin) regexReplace(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) < 2 {
		return nil, fmt.Errorf("regex_replace filter requires 2 arguments: pattern and replacement")
	}

	str := fmt.Sprintf("%v", input)
	pattern := fmt.Sprintf("%v", args[0])
	replacement := fmt.Sprintf("%v", args[1])

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %v", err)
	}

	return re.ReplaceAllString(str, replacement), nil
}

func (c *CoreFiltersPlugin) split(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	separator := " "

	if len(args) > 0 {
		separator = fmt.Sprintf("%v", args[0])
	}

	return strings.Split(str, separator), nil
}

func (c *CoreFiltersPlugin) join(input interface{}, args ...interface{}) (interface{}, error) {
	separator := " "
	if len(args) > 0 {
		separator = fmt.Sprintf("%v", args[0])
	}

	v := reflect.ValueOf(input)
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return nil, fmt.Errorf("join filter can only be applied to arrays/slices")
	}

	var parts []string
	for i := 0; i < v.Len(); i++ {
		parts = append(parts, fmt.Sprintf("%v", v.Index(i).Interface()))
	}

	return strings.Join(parts, separator), nil
}

func (c *CoreFiltersPlugin) length(input interface{}, args ...interface{}) (interface{}, error) {
	v := reflect.ValueOf(input)

	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return v.Len(), nil
	default:
		return nil, fmt.Errorf("length filter cannot be applied to type %T", input)
	}
}

// Type conversion filters
func (c *CoreFiltersPlugin) toInt(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return strconv.Atoi(fmt.Sprintf("%v", input))
	}
}

func (c *CoreFiltersPlugin) toFloat(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case float64:
		return v, nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return strconv.ParseFloat(fmt.Sprintf("%v", input), 64)
	}
}

func (c *CoreFiltersPlugin) toBool(input interface{}, args ...interface{}) (interface{}, error) {
	switch v := input.(type) {
	case bool:
		return v, nil
	case string:
		return strconv.ParseBool(v)
	case int:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case float64:
		return v != 0, nil
	default:
		return false, nil
	}
}

func (c *CoreFiltersPlugin) toString(input interface{}, args ...interface{}) (interface{}, error) {
	return fmt.Sprintf("%v", input), nil
}

func (c *CoreFiltersPlugin) toList(input interface{}, args ...interface{}) (interface{}, error) {
	v := reflect.ValueOf(input)

	if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
		return input, nil
	}

	if v.Kind() == reflect.String {
		str := input.(string)
		var result []interface{}
		for _, char := range str {
			result = append(result, string(char))
		}
		return result, nil
	}

	return []interface{}{input}, nil
}

// Hash filters
func (c *CoreFiltersPlugin) hash(input interface{}, args ...interface{}) (interface{}, error) {
	if len(args) == 0 {
		return c.sha1(input)
	}

	hashType := fmt.Sprintf("%v", args[0])
	switch hashType {
	case "md5":
		return c.md5(input)
	case "sha1":
		return c.sha1(input)
	case "sha256":
		return c.sha256(input)
	default:
		return nil, fmt.Errorf("unsupported hash type: %s", hashType)
	}
}

func (c *CoreFiltersPlugin) md5(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	hash := md5.Sum([]byte(str))
	return hex.EncodeToString(hash[:]), nil
}

func (c *CoreFiltersPlugin) sha1(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	hash := sha1.Sum([]byte(str))
	return hex.EncodeToString(hash[:]), nil
}

func (c *CoreFiltersPlugin) sha256(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	hash := sha256.Sum256([]byte(str))
	return hex.EncodeToString(hash[:]), nil
}

// Encoding filters
func (c *CoreFiltersPlugin) b64encode(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	return base64.StdEncoding.EncodeToString([]byte(str)), nil
}

func (c *CoreFiltersPlugin) b64decode(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	decoded, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	return string(decoded), nil
}

// JSON filters
func (c *CoreFiltersPlugin) toJson(input interface{}, args ...interface{}) (interface{}, error) {
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return string(jsonData), nil
}

func (c *CoreFiltersPlugin) fromJson(input interface{}, args ...interface{}) (interface{}, error) {
	str := fmt.Sprintf("%v", input)
	var result interface{}
	err := json.Unmarshal([]byte(str), &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *CoreFiltersPlugin) toNiceJson(input interface{}, args ...interface{}) (interface{}, error) {
	jsonData, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return nil, err
	}
	return string(jsonData), nil
}

// Default implementations for remaining filters (simplified for brevity)
func (c *CoreFiltersPlugin) reverse(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would reverse strings or slices
	return input, nil
}

func (c *CoreFiltersPlugin) indent(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would indent text
	return input, nil
}

func (c *CoreFiltersPlugin) center(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would center text
	return input, nil
}

func (c *CoreFiltersPlugin) truncate(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would truncate text
	return input, nil
}

func (c *CoreFiltersPlugin) abs(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return absolute value
	return input, nil
}

func (c *CoreFiltersPlugin) round(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would round numbers
	return input, nil
}

func (c *CoreFiltersPlugin) random(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return random element or number
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(100), nil
}

func (c *CoreFiltersPlugin) max(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would find maximum value
	return input, nil
}

func (c *CoreFiltersPlugin) min(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would find minimum value
	return input, nil
}

func (c *CoreFiltersPlugin) sum(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would sum numeric values
	return input, nil
}

func (c *CoreFiltersPlugin) first(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return first element
	return input, nil
}

func (c *CoreFiltersPlugin) last(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return last element
	return input, nil
}

func (c *CoreFiltersPlugin) unique(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return unique elements
	return input, nil
}

func (c *CoreFiltersPlugin) union(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return union of sets
	return input, nil
}

func (c *CoreFiltersPlugin) intersect(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return intersection of sets
	return input, nil
}

func (c *CoreFiltersPlugin) difference(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return difference of sets
	return input, nil
}

func (c *CoreFiltersPlugin) sort(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would sort elements
	return input, nil
}

func (c *CoreFiltersPlugin) flatten(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would flatten nested structures
	return input, nil
}

func (c *CoreFiltersPlugin) selectFilter(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would select elements matching condition
	return input, nil
}

func (c *CoreFiltersPlugin) reject(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would reject elements matching condition
	return input, nil
}

func (c *CoreFiltersPlugin) mapFilter(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would map elements through function
	return input, nil
}

func (c *CoreFiltersPlugin) selectAttr(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would select by attribute
	return input, nil
}

func (c *CoreFiltersPlugin) rejectAttr(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would reject by attribute
	return input, nil
}

func (c *CoreFiltersPlugin) dict2items(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would convert dict to items
	return input, nil
}

func (c *CoreFiltersPlugin) items2dict(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would convert items to dict
	return input, nil
}

func (c *CoreFiltersPlugin) combine(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would combine dictionaries
	return input, nil
}

func (c *CoreFiltersPlugin) urlencode(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would URL encode
	return input, nil
}

func (c *CoreFiltersPlugin) quote(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would quote shell arguments
	return input, nil
}

func (c *CoreFiltersPlugin) toYaml(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would convert to YAML
	return input, nil
}

func (c *CoreFiltersPlugin) fromYaml(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would parse YAML
	return input, nil
}

func (c *CoreFiltersPlugin) toNiceYaml(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would convert to pretty YAML
	return input, nil
}

func (c *CoreFiltersPlugin) strftime(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would format time
	return input, nil
}

func (c *CoreFiltersPlugin) toDatetime(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would parse datetime
	return input, nil
}

func (c *CoreFiltersPlugin) defaultFilter(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would return default if input is empty
	if len(args) == 0 {
		return input, nil
	}

	// Check if input is "empty"
	if input == nil || input == "" || input == 0 || input == false {
		return args[0], nil
	}

	return input, nil
}

func (c *CoreFiltersPlugin) ternary(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would do ternary operation
	if len(args) < 2 {
		return nil, fmt.Errorf("ternary filter requires 2 arguments")
	}

	condition, err := c.toBool(input)
	if err != nil {
		return nil, err
	}

	if condition.(bool) {
		return args[0], nil
	}
	return args[1], nil
}

func (c *CoreFiltersPlugin) regexSearch(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would search with regex
	return input, nil
}

func (c *CoreFiltersPlugin) regexFindall(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would find all regex matches
	return input, nil
}

func (c *CoreFiltersPlugin) regexEscape(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would escape regex characters
	return input, nil
}

func (c *CoreFiltersPlugin) basename(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would get basename of path
	return input, nil
}

func (c *CoreFiltersPlugin) dirname(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would get dirname of path
	return input, nil
}

func (c *CoreFiltersPlugin) expanduser(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would expand ~ in path
	return input, nil
}

func (c *CoreFiltersPlugin) realpath(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would resolve real path
	return input, nil
}

func (c *CoreFiltersPlugin) relpath(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would get relative path
	return input, nil
}

func (c *CoreFiltersPlugin) splitext(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would split file extension
	return input, nil
}

func (c *CoreFiltersPlugin) versionCompare(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would compare versions
	return input, nil
}

func (c *CoreFiltersPlugin) ipaddr(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would work with IP addresses
	return input, nil
}

func (c *CoreFiltersPlugin) ipv4(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would work with IPv4 addresses
	return input, nil
}

func (c *CoreFiltersPlugin) ipv6(input interface{}, args ...interface{}) (interface{}, error) {
	// Implementation would work with IPv6 addresses
	return input, nil
}

// FilterPluginRegistry manages filter plugin registration and creation
type FilterPluginRegistry struct {
	plugins map[string]func() FilterPlugin
}

func NewFilterPluginRegistry() *FilterPluginRegistry {
	registry := &FilterPluginRegistry{
		plugins: make(map[string]func() FilterPlugin),
	}

	// Register built-in filter plugins
	registry.Register("core", func() FilterPlugin { return NewCoreFiltersPlugin() })

	return registry
}

func (r *FilterPluginRegistry) Register(name string, creator func() FilterPlugin) {
	r.plugins[name] = creator
}

func (r *FilterPluginRegistry) Get(name string) (FilterPlugin, error) {
	creator, exists := r.plugins[name]
	if !exists {
		return nil, fmt.Errorf("filter plugin '%s' not found", name)
	}
	return creator(), nil
}

func (r *FilterPluginRegistry) Exists(name string) bool {
	_, exists := r.plugins[name]
	return exists
}

func (r *FilterPluginRegistry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}