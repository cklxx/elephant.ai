package presets

import (
	"fmt"

	"alex/internal/agent/ports"
)

// ToolPreset defines different tool access levels for agents
type ToolPreset string

const (
	ToolPresetFull     ToolPreset = "full"
	ToolPresetReadOnly ToolPreset = "read-only"
	ToolPresetCodeOnly ToolPreset = "code-only"
	ToolPresetWebOnly  ToolPreset = "web-only"
	ToolPresetSafe     ToolPreset = "safe"
)

// ToolConfig contains tool access configuration for a preset
type ToolConfig struct {
	Name         string
	Description  string
	AllowedTools map[string]bool // nil means all tools allowed
	DeniedTools  map[string]bool // Tools explicitly denied
}

// GetToolConfig returns the tool configuration for a preset
func GetToolConfig(preset ToolPreset) (*ToolConfig, error) {
	configs := map[ToolPreset]*ToolConfig{
		ToolPresetFull: {
			Name:         "Full Access",
			Description:  "All tools available - unrestricted access",
			AllowedTools: nil, // nil means allow all
			DeniedTools:  make(map[string]bool),
		},

		ToolPresetReadOnly: {
			Name:        "Read-Only Access",
			Description: "Only read operations - no modifications allowed",
			AllowedTools: map[string]bool{
				"file_read":  true,
				"list_files": true,
				"grep":       true,
				"ripgrep":    true,
				"find":       true,
				"web_search": true,
				"web_fetch":  true,
				"think":      true,
				"todo_read":  true,
				"subagent":   true,
			},
			DeniedTools: map[string]bool{
				"file_write":   true,
				"file_edit":    true,
				"bash":         true,
				"code_execute": true,
				"todo_update":  true,
			},
		},

		ToolPresetCodeOnly: {
			Name:        "Code Operations",
			Description: "File operations and code execution - no web access",
			AllowedTools: map[string]bool{
				"file_read":    true,
				"file_write":   true,
				"file_edit":    true,
				"list_files":   true,
				"grep":         true,
				"ripgrep":      true,
				"find":         true,
				"code_execute": true,
				"think":        true,
				"todo_read":    true,
				"todo_update":  true,
				"subagent":     true,
			},
			DeniedTools: map[string]bool{
				"web_search": true,
				"web_fetch":  true,
				"bash":       true,
			},
		},

		ToolPresetWebOnly: {
			Name:        "Web Access",
			Description: "Web search and fetch only - no file system access",
			AllowedTools: map[string]bool{
				"web_search": true,
				"web_fetch":  true,
				"think":      true,
				"todo_read":  true,
			},
			DeniedTools: map[string]bool{
				"file_read":    true,
				"file_write":   true,
				"file_edit":    true,
				"list_files":   true,
				"bash":         true,
				"code_execute": true,
				"grep":         true,
				"ripgrep":      true,
				"find":         true,
				"todo_update":  true,
				"subagent":     true,
			},
		},

		ToolPresetSafe: {
			Name:        "Safe Mode",
			Description: "Excludes potentially dangerous tools (bash, code execution)",
			AllowedTools: map[string]bool{
				"file_read":   true,
				"file_write":  true,
				"file_edit":   true,
				"list_files":  true,
				"grep":        true,
				"ripgrep":     true,
				"find":        true,
				"web_search":  true,
				"web_fetch":   true,
				"think":       true,
				"todo_read":   true,
				"todo_update": true,
				"subagent":    true,
			},
			DeniedTools: map[string]bool{
				"bash":         true,
				"code_execute": true,
			},
		},
	}

	config, ok := configs[preset]
	if !ok {
		return nil, fmt.Errorf("unknown tool preset: %s", preset)
	}

	return config, nil
}

// FilteredToolRegistry wraps a tool registry with preset-based filtering
type FilteredToolRegistry struct {
	parent ports.ToolRegistry
	config *ToolConfig
}

// NewFilteredToolRegistry creates a filtered registry based on tool preset
func NewFilteredToolRegistry(parent ports.ToolRegistry, preset ToolPreset) (*FilteredToolRegistry, error) {
	config, err := GetToolConfig(preset)
	if err != nil {
		return nil, err
	}

	return &FilteredToolRegistry{
		parent: parent,
		config: config,
	}, nil
}

// Get retrieves a tool if allowed by the preset
func (f *FilteredToolRegistry) Get(name string) (ports.ToolExecutor, error) {
	// Check if tool is denied
	if f.config.DeniedTools[name] {
		return nil, fmt.Errorf("tool not available in %s preset: %s", f.config.Name, name)
	}

	// If AllowedTools is nil, all tools are allowed (unless explicitly denied)
	if f.config.AllowedTools == nil {
		return f.parent.Get(name)
	}

	// Check if tool is in allowed list
	if !f.config.AllowedTools[name] {
		return nil, fmt.Errorf("tool not available in %s preset: %s", f.config.Name, name)
	}

	return f.parent.Get(name)
}

// List returns only tools allowed by the preset
func (f *FilteredToolRegistry) List() []ports.ToolDefinition {
	allTools := f.parent.List()
	filtered := make([]ports.ToolDefinition, 0)

	for _, tool := range allTools {
		// Skip denied tools
		if f.config.DeniedTools[tool.Name] {
			continue
		}

		// If AllowedTools is nil, include all (unless denied)
		if f.config.AllowedTools == nil {
			filtered = append(filtered, tool)
			continue
		}

		// Include only allowed tools
		if f.config.AllowedTools[tool.Name] {
			filtered = append(filtered, tool)
		}
	}

	return filtered
}

// Register delegates to parent registry
func (f *FilteredToolRegistry) Register(tool ports.ToolExecutor) error {
	return f.parent.Register(tool)
}

// Unregister delegates to parent registry
func (f *FilteredToolRegistry) Unregister(name string) error {
	return f.parent.Unregister(name)
}

// GetAllToolPresets returns all available tool presets
func GetAllToolPresets() []ToolPreset {
	return []ToolPreset{
		ToolPresetFull,
		ToolPresetReadOnly,
		ToolPresetCodeOnly,
		ToolPresetWebOnly,
		ToolPresetSafe,
	}
}

// IsValidToolPreset checks if a tool preset is valid
func IsValidToolPreset(preset string) bool {
	switch ToolPreset(preset) {
	case ToolPresetFull, ToolPresetReadOnly, ToolPresetCodeOnly, ToolPresetWebOnly, ToolPresetSafe:
		return true
	default:
		return false
	}
}
