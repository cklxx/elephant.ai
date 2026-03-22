package presets

import (
	"fmt"
	"strings"

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
	ToolPresetArchitect ToolPreset = "architect"
)

// ToolConfig contains tool access configuration for a preset.
// All presets currently grant unrestricted access; the struct is
// retained so callers that inspect config.Name continue to work.
type ToolConfig struct {
	Name        string
	Description string
}

// NormalizeToolMode trims and normalizes tool mode values, defaulting to CLI.
func NormalizeToolMode(raw string) ToolMode {
	mode := ToolMode(strings.TrimSpace(raw))
	if mode == "" {
		return ToolModeCLI
	}
	return mode
}

// DefaultToolPresetForMode trims preset names and applies per-mode defaults.
func DefaultToolPresetForMode(mode ToolMode, preset string) string {
	trimmed := strings.TrimSpace(preset)
	if NormalizeToolMode(string(mode)) == ToolModeCLI && trimmed == "" {
		return string(ToolPresetFull)
	}
	return trimmed
}

// GetToolConfig returns the tool configuration for a mode and preset.
// All valid combinations return unrestricted access.
func GetToolConfig(mode ToolMode, preset ToolPreset) (*ToolConfig, error) {
	if mode == "" {
		mode = ToolModeCLI
	}
	switch mode {
	case ToolModeWeb:
		switch preset {
		case "", ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe, ToolPresetArchitect:
			name := "Web Mode"
			if preset == ToolPresetArchitect {
				name = "Architect Access"
			}
			return &ToolConfig{Name: name, Description: "Unrestricted tool access"}, nil
		default:
			return nil, fmt.Errorf("unknown tool preset: %s", preset)
		}
	case ToolModeCLI:
		if preset == "" {
			preset = ToolPresetFull
		}
		switch preset {
		case ToolPresetFull:
			return &ToolConfig{Name: "Full Access", Description: "All tools available - unrestricted access"}, nil
		case ToolPresetReadOnly:
			return &ToolConfig{Name: "Read-Only Access", Description: "All tools available - preset label retained for compatibility"}, nil
		case ToolPresetSafe:
			return &ToolConfig{Name: "Safe Mode", Description: "All tools available - preset label retained for compatibility"}, nil
		case ToolPresetArchitect:
			return &ToolConfig{Name: "Architect Access", Description: "All tools available - preset label retained for compatibility"}, nil
		default:
			return nil, fmt.Errorf("unknown tool preset: %s", preset)
		}
	default:
		return nil, fmt.Errorf("unknown tool mode: %s", mode)
	}
}

// NewFilteredToolRegistry returns the parent registry unchanged because all
// presets grant unrestricted tool access. The function is kept for API
// compatibility with existing callers.
func NewFilteredToolRegistry(parent tools.ToolRegistry, mode ToolMode, preset ToolPreset) (tools.ToolRegistry, error) {
	if _, err := GetToolConfig(mode, preset); err != nil {
		return nil, err
	}
	return parent, nil
}

// IsValidToolPreset checks if a tool preset is valid
func IsValidToolPreset(preset string) bool {
	switch ToolPreset(preset) {
	case ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe, ToolPresetArchitect:
		return true
	default:
		return false
	}
}
