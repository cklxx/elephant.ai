package presets

import (
	"fmt"

	"alex/internal/agent/ports"
)

// ToolMode defines the runtime surface the agent runs under.
type ToolMode string

const (
	ToolModeCLI ToolMode = "cli"
	ToolModeWeb ToolMode = "web"
)

// ToolPreset defines tool access levels for CLI agents.
type ToolPreset string

const (
	ToolPresetFull     ToolPreset = "full"
	ToolPresetReadOnly ToolPreset = "read-only"
	ToolPresetSafe     ToolPreset = "safe"
)

// ToolConfig contains tool access configuration for a preset
type ToolConfig struct {
	Name         string
	Description  string
	AllowedTools map[string]bool // nil means all tools allowed
	DeniedTools  map[string]bool // Tools explicitly denied
}

var (
	cliDeniedTools = map[string]bool{
		"artifacts_write":  true,
		"artifacts_list":   true,
		"artifacts_delete": true,
	}
	webDeniedTools = map[string]bool{
		"file_read":    true,
		"file_write":   true,
		"file_edit":    true,
		"list_files":   true,
		"grep":         true,
		"ripgrep":      true,
		"find":         true,
		"bash":         true,
		"code_execute": true,
		"skills":       true,
		"todo_read":    true,
		"todo_update":  true,
	}
	readOnlyDeniedTools = map[string]bool{
		"file_write":   true,
		"file_edit":    true,
		"bash":         true,
		"code_execute": true,
		"todo_update":  true,
	}
	safeDeniedTools = map[string]bool{
		"bash":         true,
		"code_execute": true,
	}
)

func cloneToolSet(src map[string]bool) map[string]bool {
	dst := make(map[string]bool, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func mergeToolSets(dst map[string]bool, src map[string]bool) map[string]bool {
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

// GetToolConfig returns the tool configuration for a mode and preset.
func GetToolConfig(mode ToolMode, preset ToolPreset) (*ToolConfig, error) {
	if mode == "" {
		mode = ToolModeCLI
	}
	switch mode {
	case ToolModeWeb:
		return &ToolConfig{
			Name:         "Web Mode",
			Description:  "All non-local tools (file/shell/code exec disabled)",
			AllowedTools: nil,
			DeniedTools:  cloneToolSet(webDeniedTools),
		}, nil
	case ToolModeCLI:
		if preset == "" {
			preset = ToolPresetFull
		}
		switch preset {
		case ToolPresetFull:
			return &ToolConfig{
				Name:         "Full Access",
				Description:  "All tools available - unrestricted access",
				AllowedTools: nil,
				DeniedTools:  cloneToolSet(cliDeniedTools),
			}, nil
		case ToolPresetReadOnly:
			return &ToolConfig{
				Name:         "Read-Only Access",
				Description:  "No local writes or shell/code execution",
				AllowedTools: nil,
				DeniedTools:  mergeToolSets(cloneToolSet(readOnlyDeniedTools), cliDeniedTools),
			}, nil
		case ToolPresetSafe:
			return &ToolConfig{
				Name:         "Safe Mode",
				Description:  "Excludes potentially dangerous tools (bash, code execution)",
				AllowedTools: nil,
				DeniedTools:  mergeToolSets(cloneToolSet(safeDeniedTools), cliDeniedTools),
			}, nil
		default:
			return nil, fmt.Errorf("unknown tool preset: %s", preset)
		}
	default:
		return nil, fmt.Errorf("unknown tool mode: %s", mode)
	}
}

// FilteredToolRegistry wraps a tool registry with preset-based filtering
type FilteredToolRegistry struct {
	parent ports.ToolRegistry
	config *ToolConfig
}

// NewFilteredToolRegistry creates a filtered registry based on tool mode and preset.
func NewFilteredToolRegistry(parent ports.ToolRegistry, mode ToolMode, preset ToolPreset) (*FilteredToolRegistry, error) {
	config, err := GetToolConfig(mode, preset)
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
		ToolPresetSafe,
	}
}

// IsValidToolPreset checks if a tool preset is valid
func IsValidToolPreset(preset string) bool {
	switch ToolPreset(preset) {
	case ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe:
		return true
	default:
		return false
	}
}
