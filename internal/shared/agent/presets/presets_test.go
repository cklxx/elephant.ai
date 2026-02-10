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
			name:    "lark-local preset",
			mode:    ToolModeCLI,
			preset:  ToolPresetLarkLocal,
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
			name:    "web mode lark-local preset",
			mode:    ToolModeWeb,
			preset:  ToolPresetLarkLocal,
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

func TestIsValidPreset(t *testing.T) {
	tests := []struct {
		name   string
		preset string
		want   bool
	}{
		{"default", "default", true},
		{"code-expert", "code-expert", true},
		{"researcher", "researcher", true},
		{"devops", "devops", true},
		{"security-analyst", "security-analyst", true},
		{"architect", "architect", true},
		{"invalid", "invalid", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidPreset(tt.preset); got != tt.want {
				t.Errorf("IsValidPreset() = %v, want %v", got, tt.want)
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
		{"sandbox", "sandbox", true},
		{"architect", "architect", true},
		{"lark-local", "lark-local", true},
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

func TestDefaultPromptIncludesRoutingGuardrails(t *testing.T) {
	t.Parallel()

	config, err := GetPromptConfig(PresetDefault)
	if err != nil {
		t.Fatalf("GetPromptConfig() error = %v", err)
	}
	prompt := config.SystemPrompt
	for _, snippet := range []string{
		"Use `request_user` for explicit human approval/consent/manual gates",
		"Exhaust safe deterministic attempts before asking the user",
		"If intent is unclear, inspect memory and thread context first",
		"Distinguish repo files vs memory notes",
		"Prefer `execute_code` for deterministic recalculation/metric/invariant checks",
		"Default temporary/generated files to `/tmp`",
		"search/install suitable skills or tools from trusted sources",
		"Use `channel` for all Lark operations",
		"generated file is part of the requested deliverable",
		"Use `browser_action` for browser interactions",
		"Use `skills` to invoke declarative skill workflows",
	} {
		if !strings.Contains(prompt, snippet) {
			t.Fatalf("expected system prompt to contain %q", snippet)
		}
	}
}

func TestToolPresetBlocking(t *testing.T) {
	tests := []struct {
		name      string
		mode      ToolMode
		preset    ToolPreset
		toolName  string
		wantAllow bool
	}{
		{
			name:      "read-only allows file_read",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "file_read",
			wantAllow: true,
		},
		{
			name:      "read-only allows file_write",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "file_write",
			wantAllow: true,
		},
		{
			name:      "read-only allows bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "bash",
			wantAllow: true,
		},
		{
			name:      "safe allows file_write",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "file_write",
			wantAllow: true,
		},
		{
			name:      "safe allows bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "bash",
			wantAllow: true,
		},
		{
			name:      "safe allows code_execute",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "code_execute",
			wantAllow: true,
		},
		{
			name:      "full allows channel in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "channel",
			wantAllow: true,
		},
		{
			name:      "full allows web_search in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "web_search",
			wantAllow: true,
		},
		{
			name:      "full allows shell_exec in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "shell_exec",
			wantAllow: true,
		},
		{
			name:      "full allows execute_code in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "execute_code",
			wantAllow: true,
		},
		{
			name:      "read-only allows read_file",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "read_file",
			wantAllow: true,
		},
		{
			name:      "safe allows read_file",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "read_file",
			wantAllow: true,
		},
		{
			name:      "read-only allows plan",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "plan",
			wantAllow: true,
		},
		{
			name:      "read-only allows clarify",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "clarify",
			wantAllow: true,
		},
		{
			name:      "safe allows plan",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "plan",
			wantAllow: true,
		},
		{
			name:      "safe allows clarify",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "clarify",
			wantAllow: true,
		},
		{
			name:      "sandbox allows file_read",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "file_read",
			wantAllow: true,
		},
		{
			name:      "sandbox allows bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "bash",
			wantAllow: true,
		},
		{
			name:      "sandbox allows shell_exec",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "shell_exec",
			wantAllow: true,
		},
		{
			name:      "sandbox allows execute_code",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "execute_code",
			wantAllow: true,
		},
		{
			name:      "lark-local allows read_file",
			mode:      ToolModeCLI,
			preset:    ToolPresetLarkLocal,
			toolName:  "read_file",
			wantAllow: true,
		},
		{
			name:      "lark-local allows browser_action",
			mode:      ToolModeCLI,
			preset:    ToolPresetLarkLocal,
			toolName:  "browser_action",
			wantAllow: true,
		},
		{
			name:      "lark-local allows channel",
			mode:      ToolModeCLI,
			preset:    ToolPresetLarkLocal,
			toolName:  "channel",
			wantAllow: true,
		},
		{
			name:      "web mode allows file_read",
			mode:      ToolModeWeb,
			preset:    ToolPresetFull,
			toolName:  "file_read",
			wantAllow: true,
		},
		{
			name:      "web mode allows web_search",
			mode:      ToolModeWeb,
			preset:    ToolPresetFull,
			toolName:  "web_search",
			wantAllow: true,
		},
		{
			name:      "web mode allows skills",
			mode:      ToolModeWeb,
			preset:    ToolPresetFull,
			toolName:  "skills",
			wantAllow: true,
		},
		{
			name:      "architect allows web_search",
			mode:      ToolModeCLI,
			preset:    ToolPresetArchitect,
			toolName:  "web_search",
			wantAllow: true,
		},
		{
			name:      "architect allows plan in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetArchitect,
			toolName:  "plan",
			wantAllow: true,
		},
		{
			name:      "architect allows bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetArchitect,
			toolName:  "bash",
			wantAllow: true,
		},
		{
			name:      "web mode architect allows plan",
			mode:      ToolModeWeb,
			preset:    ToolPresetArchitect,
			toolName:  "plan",
			wantAllow: true,
		},
		{
			name:      "web mode architect allows file_read",
			mode:      ToolModeWeb,
			preset:    ToolPresetArchitect,
			toolName:  "file_read",
			wantAllow: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := GetToolConfig(tt.mode, tt.preset)
			if err != nil {
				t.Fatalf("GetToolConfig() error = %v", err)
			}

			// Check if tool is blocked
			isBlocked := config.DeniedTools[tt.toolName]

			// Check if tool is allowed (if AllowedTools is not nil)
			isAllowed := false
			if config.AllowedTools == nil {
				// nil means allow all (unless denied)
				isAllowed = !isBlocked
			} else {
				isAllowed = config.AllowedTools[tt.toolName]
			}

			if isAllowed != tt.wantAllow {
				t.Errorf("Tool %s allowed = %v, want %v (preset: %s)", tt.toolName, isAllowed, tt.wantAllow, tt.preset)
			}
		})
	}
}

func TestGetAllPresets(t *testing.T) {
	presets := GetAllPresets()
	if len(presets) != 7 {
		t.Errorf("GetAllPresets() returned %d presets, want 7", len(presets))
	}

	// Check all expected presets are present
	expected := map[AgentPreset]bool{
		PresetDefault:         false,
		PresetCodeExpert:      false,
		PresetResearcher:      false,
		PresetDevOps:          false,
		PresetSecurityAnalyst: false,
		PresetDesigner:        false,
		PresetArchitect:       false,
	}

	for _, preset := range presets {
		if _, ok := expected[preset]; ok {
			expected[preset] = true
		} else {
			t.Errorf("Unexpected preset: %s", preset)
		}
	}

	for preset, found := range expected {
		if !found {
			t.Errorf("Missing preset: %s", preset)
		}
	}
}

func TestGetAllToolPresets(t *testing.T) {
	presets := GetAllToolPresets()
	if len(presets) != 6 {
		t.Errorf("GetAllToolPresets() returned %d presets, want 6", len(presets))
	}

	// Check all expected presets are present
	expected := map[ToolPreset]bool{
		ToolPresetFull:      false,
		ToolPresetReadOnly:  false,
		ToolPresetSafe:      false,
		ToolPresetSandbox:   false,
		ToolPresetArchitect: false,
		ToolPresetLarkLocal: false,
	}

	for _, preset := range presets {
		if _, ok := expected[preset]; ok {
			expected[preset] = true
		} else {
			t.Errorf("Unexpected tool preset: %s", preset)
		}
	}

	for preset, found := range expected {
		if !found {
			t.Errorf("Missing tool preset: %s", preset)
		}
	}
}
