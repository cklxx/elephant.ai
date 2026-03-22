package presets

import (
	"strings"
	"testing"
)

func TestGetPromptConfig(t *testing.T) {
	tests := []struct {
		name    string
		preset  AgentPreset
		wantErr bool
	}{
		{
			name:    "default preset",
			preset:  PresetDefault,
			wantErr: false,
		},
		{
			name:    "code-expert preset",
			preset:  PresetCodeExpert,
			wantErr: false,
		},
		{
			name:    "researcher preset",
			preset:  PresetResearcher,
			wantErr: false,
		},
		{
			name:    "devops preset",
			preset:  PresetDevOps,
			wantErr: false,
		},
		{
			name:    "security-analyst preset",
			preset:  PresetSecurityAnalyst,
			wantErr: false,
		},
		{
			name:    "architect preset",
			preset:  PresetArchitect,
			wantErr: false,
		},
		{
			name:    "invalid preset",
			preset:  AgentPreset("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetPromptConfig(tt.preset)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetPromptConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if config == nil {
					t.Error("GetPromptConfig() returned nil config")
					return
				}
				if config.Name == "" {
					t.Error("GetPromptConfig() returned empty name")
				}
				if config.SystemPrompt == "" {
					t.Error("GetPromptConfig() returned empty system prompt")
				}
			}
		})
	}
}

func TestGetToolConfig(t *testing.T) {
	tests := []struct {
		name    string
		mode    ToolMode
		preset  ToolPreset
		wantErr bool
	}{
		{
			name:    "full preset",
			mode:    ToolModeCLI,
			preset:  ToolPresetFull,
			wantErr: false,
		},
		{
			name:    "read-only preset",
			mode:    ToolModeCLI,
			preset:  ToolPresetReadOnly,
			wantErr: false,
		},
		{
			name:    "safe preset",
			mode:    ToolModeCLI,
			preset:  ToolPresetSafe,
			wantErr: false,
		},
		{
			name:    "architect preset",
			mode:    ToolModeCLI,
			preset:  ToolPresetArchitect,
			wantErr: false,
		},
		{
			name:    "web mode default",
			mode:    ToolModeWeb,
			preset:  ToolPreset(""),
			wantErr: false,
		},
		{
			name:    "web mode architect preset",
			mode:    ToolModeWeb,
			preset:  ToolPresetArchitect,
			wantErr: false,
		},
		{
			name:    "invalid preset",
			mode:    ToolModeCLI,
			preset:  ToolPreset("invalid"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetToolConfig(tt.mode, tt.preset)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetToolConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if config == nil {
					t.Error("GetToolConfig() returned nil config")
					return
				}
				if config.Name == "" {
					t.Error("GetToolConfig() returned empty name")
				}
			}
		})
	}
}

func TestIsValidToolPreset(t *testing.T) {
	tests := []struct {
		name   string
		preset string
		want   bool
	}{
		{"full", "full", true},
		{"read-only", "read-only", true},
		{"safe", "safe", true},
		{"architect", "architect", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidToolPreset(tt.preset); got != tt.want {
				t.Errorf("IsValidToolPreset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeToolMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want ToolMode
	}{
		{name: "empty defaults to cli", raw: "", want: ToolModeCLI},
		{name: "trimmed web", raw: "  web  ", want: ToolModeWeb},
		{name: "preserves explicit mode", raw: "CLI", want: ToolMode("CLI")},
		{name: "preserves unknown mode", raw: "desktop", want: ToolMode("desktop")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeToolMode(tt.raw); got != tt.want {
				t.Fatalf("NormalizeToolMode(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestDefaultToolPresetForMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mode   ToolMode
		preset string
		want   string
	}{
		{name: "cli empty defaults to full", mode: ToolModeCLI, preset: "", want: "full"},
		{name: "cli keeps non-empty", mode: ToolModeCLI, preset: "safe", want: "safe"},
		{name: "web keeps empty", mode: ToolModeWeb, preset: "", want: ""},
		{name: "trim preset", mode: ToolModeWeb, preset: "  architect  ", want: "architect"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultToolPresetForMode(tt.mode, tt.preset); got != tt.want {
				t.Fatalf("DefaultToolPresetForMode(%q, %q) = %q, want %q", tt.mode, tt.preset, got, tt.want)
			}
		})
	}
}

func TestDefaultPromptIncludesRoutingGuardrails(t *testing.T) {
	t.Parallel()

	config, err := GetPromptConfig(PresetDefault)
	if err != nil {
		t.Fatalf("GetPromptConfig() error = %v", err)
	}
	prompt := config.SystemPrompt
	for _, snippet := range []string{
		// 7C Response Quality with NEVER patterns
		"NEVER invent facts",
		"NEVER bury the conclusion",
		"NEVER repeat information already stated",
		"NEVER silently drop requirements",
		// Response style forbidden patterns
		"NEVER start responses with filler",
		// Tool routing reinforcement (slimmed to 3 key rules)
		"ONLY when critical input is missing after all viable tool attempts fail",
		"ONLY for explicit human gates (login, 2FA, CAPTCHA",
		"NEVER for single-step actions",
	} {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected system prompt to contain %q", snippet)
		}
	}
}

func TestToolPresetAllowsAllTools(t *testing.T) {
	// All presets now grant unrestricted access. Verify GetToolConfig
	// succeeds for all valid mode+preset combinations.
	modes := []ToolMode{ToolModeCLI, ToolModeWeb}
	presets := []ToolPreset{ToolPresetFull, ToolPresetReadOnly, ToolPresetSafe, ToolPresetArchitect}

	for _, mode := range modes {
		for _, preset := range presets {
			t.Run(string(mode)+"/"+string(preset), func(t *testing.T) {
				config, err := GetToolConfig(mode, preset)
				if err != nil {
					t.Fatalf("GetToolConfig() error = %v", err)
				}
				if config.Name == "" {
					t.Error("expected non-empty config name")
				}
			})
		}
	}
}
