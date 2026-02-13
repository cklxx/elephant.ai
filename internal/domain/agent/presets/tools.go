package presets

import (
	"fmt"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

// ToolMode defines the runtime surface the agent runs under.
type ToolMode string

const (
	ToolModeCLI ToolMode = "cli"
	ToolModeWeb ToolMode = "web"
)

// ToolPreset defines tool access levels for CLI and web agents.
type ToolPreset string

const (
	ToolPresetFull      ToolPreset = "full"
	ToolPresetReadOnly  ToolPreset = "read-only"
	ToolPresetSafe      ToolPreset = "safe"
	ToolPresetSandbox   ToolPreset = "sandbox"
	ToolPresetArchitect ToolPreset = "architect"
	ToolPresetLarkLocal ToolPreset = "lark-local"
)

// ToolConfig contains tool access configuration for a preset
type ToolConfig struct {
	Name         string
	Description  string
	AllowedTools map[string]bool // nil means all tools allowed
	DeniedTools  map[string]bool // Tools explicitly denied
}

func unrestrictedToolConfig(name, description string) *ToolConfig {
	return &ToolConfig{
		Name:         name,
		Description:  description,
		AllowedTools: nil,
		DeniedTools:  map[string]bool{},
	}
}

// GetToolConfig returns the tool configuration for a mode and preset.
func GetToolConfig(mode ToolMode, preset ToolPreset) (*ToolConfig, error) {
	if mode == "" {
		mode = ToolModeCLI
	}
	switch mode {
	case ToolModeWeb:
		if preset == "" {
			return unrestrictedToolConfig(
				"Web Mode",
				"Unrestricted tool access for web mode",
			), nil
		}
		switch preset {
		case ToolPresetArchitect:
			return unrestrictedToolConfig(
				"Architect Access",
				"Unrestricted tool access for architect preset in web mode",
			), nil
		case ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe, ToolPresetSandbox:
			return unrestrictedToolConfig(
				"Web Mode",
				"Unrestricted tool access for web mode",
			), nil
		case ToolPresetLarkLocal:
			return unrestrictedToolConfig(
				"Lark Local",
				"Unrestricted tool access for lark-local preset in web mode",
			), nil
		default:
			return nil, fmt.Errorf("unknown tool preset: %s", preset)
		}
	case ToolModeCLI:
		if preset == "" {
			preset = ToolPresetFull
		}
		switch preset {
		case ToolPresetFull:
			return unrestrictedToolConfig(
				"Full Access",
				"All tools available - unrestricted access",
			), nil
		case ToolPresetReadOnly:
			return unrestrictedToolConfig(
				"Read-Only Access",
				"All tools available - preset label retained for compatibility",
			), nil
		case ToolPresetSafe:
			return unrestrictedToolConfig(
				"Safe Mode",
				"All tools available - preset label retained for compatibility",
			), nil
		case ToolPresetSandbox:
			return unrestrictedToolConfig(
				"Sandbox Access",
				"All tools available - preset label retained for compatibility",
			), nil
		case ToolPresetArchitect:
			return unrestrictedToolConfig(
				"Architect Access",
				"All tools available - preset label retained for compatibility",
			), nil
		case ToolPresetLarkLocal:
			return unrestrictedToolConfig(
				"Lark Local",
				"All tools available - preset label retained for compatibility",
			), nil
		default:
			return nil, fmt.Errorf("unknown tool preset: %s", preset)
		}
	default:
		return nil, fmt.Errorf("unknown tool mode: %s", mode)
	}
}

// FilteredToolRegistry wraps a tool registry with preset-based filtering
type FilteredToolRegistry struct {
	parent tools.ToolRegistry
	config *ToolConfig
}

// NewFilteredToolRegistry creates a filtered registry based on tool mode and preset.
func NewFilteredToolRegistry(parent tools.ToolRegistry, mode ToolMode, preset ToolPreset) (*FilteredToolRegistry, error) {
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
func (f *FilteredToolRegistry) Get(name string) (tools.ToolExecutor, error) {
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
func (f *FilteredToolRegistry) Register(tool tools.ToolExecutor) error {
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
		ToolPresetSandbox,
		ToolPresetArchitect,
		ToolPresetLarkLocal,
	}
}

// IsValidToolPreset checks if a tool preset is valid
func IsValidToolPreset(preset string) bool {
	switch ToolPreset(preset) {
	case ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe, ToolPresetSandbox, ToolPresetArchitect, ToolPresetLarkLocal:
		return true
	default:
		return false
	}
}
