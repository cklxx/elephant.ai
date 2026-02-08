package config

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"

	"gopkg.in/yaml.v3"
)

// sensitiveKeys is the blocklist of keys that cannot be written via the config tool.
var sensitiveKeys = map[string]bool{
	"api_key":         true,
	"ark_api_key":     true,
	"tavily_api_key":  true,
	"moltbook_api_key": true,
	"app_secret":      true,
}

type configManage struct {
	shared.BaseTool
}

// NewConfigManage returns a tool executor for reading and modifying runtime config.
func NewConfigManage() tools.ToolExecutor {
	return &configManage{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "config_manage",
				Description: `Read or modify the agent's runtime configuration.

Actions:
- get: Read a specific config key or full config. Returns YAML.
- set: Set a runtime config key. Persists to the YAML config file.
- list: List all configurable runtime keys with current values.

Sensitive keys (api_key, ark_api_key, tavily_api_key, moltbook_api_key, app_secret) cannot be written.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: `Action to perform: "get", "set", or "list".`,
							Enum:        []any{"get", "set", "list"},
						},
						"key": {
							Type:        "string",
							Description: "Config key (yaml field name, e.g. max_iterations, llm_provider). Required for get/set.",
						},
						"value": {
							Type:        "string",
							Description: "New value to set. Required for set action.",
						},
					},
					Required: []string{"action"},
				},
			},
			ports.ToolMetadata{
				Name:        "config_manage",
				Version:     "1.0.0",
				Category:    "config",
				Tags:        []string{"config", "write"},
				SafetyLevel: 2,
				Dangerous:   true,
			},
		),
	}
}

func (t *configManage) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action := strings.ToLower(strings.TrimSpace(shared.StringArg(call.Arguments, "action")))
	key := strings.TrimSpace(shared.StringArg(call.Arguments, "key"))
	value := strings.TrimSpace(shared.StringArg(call.Arguments, "value"))

	switch action {
	case "get":
		return t.handleGet(call.ID, key)
	case "set":
		return t.handleSet(call.ID, key, value)
	case "list":
		return t.handleList(call.ID)
	default:
		return shared.ToolError(call.ID, "action must be get, set, or list")
	}
}

func (t *configManage) handleGet(callID, key string) (*ports.ToolResult, error) {
	cfg, _, _ := runtimeconfig.Load()
	if key == "" {
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return shared.ToolError(callID, "marshal config: %v", err)
		}
		return &ports.ToolResult{CallID: callID, Content: string(data)}, nil
	}
	val, ok := runtimeFieldByYAMLTag(cfg, key)
	if !ok {
		return shared.ToolError(callID, "unknown config key: %s", key)
	}
	return &ports.ToolResult{CallID: callID, Content: fmt.Sprintf("%s: %v", key, val)}, nil
}

func (t *configManage) handleSet(callID, key, value string) (*ports.ToolResult, error) {
	if key == "" {
		return shared.ToolError(callID, "key is required for set action")
	}
	if sensitiveKeys[key] {
		return shared.ToolError(callID, "key %q is sensitive and cannot be modified via this tool", key)
	}
	if value == "" {
		return shared.ToolError(callID, "value is required for set action")
	}

	// Validate the key exists in RuntimeConfig.
	cfg, _, _ := runtimeconfig.Load()
	if _, ok := runtimeFieldByYAMLTag(cfg, key); !ok {
		return shared.ToolError(callID, "unknown config key: %s", key)
	}

	// Coerce value to the correct type.
	coerced, err := coerceValue(cfg, key, value)
	if err != nil {
		return shared.ToolError(callID, "invalid value for %s: %v", key, err)
	}

	path, err := runtimeconfig.SaveRuntimeField(key, coerced)
	if err != nil {
		return shared.ToolError(callID, "save config: %v", err)
	}
	return &ports.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("Set %s = %v (saved to %s)", key, coerced, path),
	}, nil
}

func (t *configManage) handleList(callID string) (*ports.ToolResult, error) {
	cfg, _, _ := runtimeconfig.Load()
	entries := listRuntimeFields(cfg)
	var sb strings.Builder
	sb.WriteString("Runtime configuration keys:\n")
	for _, e := range entries {
		sensitive := ""
		if sensitiveKeys[e.key] {
			sensitive = " [read-only]"
		}
		sb.WriteString(fmt.Sprintf("\n  %s: %v%s", e.key, e.value, sensitive))
	}
	return &ports.ToolResult{CallID: callID, Content: sb.String()}, nil
}

type fieldEntry struct {
	key   string
	value any
}

// runtimeFieldByYAMLTag looks up a RuntimeConfig field by its yaml tag name.
func runtimeFieldByYAMLTag(rc runtimeconfig.RuntimeConfig, key string) (any, bool) {
	v := reflect.ValueOf(rc)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag == key {
			return v.Field(i).Interface(), true
		}
	}
	return nil, false
}

// listRuntimeFields returns all RuntimeConfig fields as yaml-tag → value pairs.
func listRuntimeFields(rc runtimeconfig.RuntimeConfig) []fieldEntry {
	v := reflect.ValueOf(rc)
	t := v.Type()
	entries := make([]fieldEntry, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag == "" || tag == "-" {
			continue
		}
		entries = append(entries, fieldEntry{key: tag, value: v.Field(i).Interface()})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].key < entries[j].key })
	return entries
}

// coerceValue converts a string value to the appropriate Go type for the given config key.
func coerceValue(rc runtimeconfig.RuntimeConfig, key, value string) (any, error) {
	v := reflect.ValueOf(rc)
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		if idx := strings.Index(tag, ","); idx != -1 {
			tag = tag[:idx]
		}
		if tag != key {
			continue
		}
		switch field.Type.Kind() {
		case reflect.String:
			return value, nil
		case reflect.Bool:
			switch strings.ToLower(value) {
			case "true", "1", "yes":
				return true, nil
			case "false", "0", "no":
				return false, nil
			default:
				return nil, fmt.Errorf("expected boolean, got %q", value)
			}
		case reflect.Int, reflect.Int64:
			var n int64
			if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
				return nil, fmt.Errorf("expected integer, got %q", value)
			}
			if field.Type.Kind() == reflect.Int {
				return int(n), nil
			}
			return n, nil
		case reflect.Float64:
			var f float64
			if _, err := fmt.Sscanf(value, "%f", &f); err != nil {
				return nil, fmt.Errorf("expected float, got %q", value)
			}
			return f, nil
		default:
			return value, nil
		}
	}
	return nil, fmt.Errorf("unknown key: %s", key)
}
