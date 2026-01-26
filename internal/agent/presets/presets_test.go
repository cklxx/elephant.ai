package presets

import (
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
			name:      "read-only blocks file_write",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "file_write",
			wantAllow: false,
		},
		{
			name:      "read-only blocks bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "bash",
			wantAllow: false,
		},
		{
			name:      "safe allows file_write",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "file_write",
			wantAllow: true,
		},
		{
			name:      "safe blocks bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "bash",
			wantAllow: false,
		},
		{
			name:      "safe blocks code_execute",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "code_execute",
			wantAllow: false,
		},
		{
			name:      "full blocks artifacts_write in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "artifacts_write",
			wantAllow: false,
		},
		{
			name:      "full blocks acp_executor in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "acp_executor",
			wantAllow: false,
		},
		{
			name:      "full handles sandbox_shell_exec in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "sandbox_shell_exec",
			wantAllow: false,
		},
		{
			name:      "full handles sandbox_code_execute in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetFull,
			toolName:  "sandbox_code_execute",
			wantAllow: false,
		},
		{
			name:      "read-only allows vision_analyze",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "vision_analyze",
			wantAllow: true,
		},
		{
			name:      "safe allows vision_analyze",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "vision_analyze",
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
			name:      "read-only allows clearify",
			mode:      ToolModeCLI,
			preset:    ToolPresetReadOnly,
			toolName:  "clearify",
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
			name:      "safe allows clearify",
			mode:      ToolModeCLI,
			preset:    ToolPresetSafe,
			toolName:  "clearify",
			wantAllow: true,
		},
		{
			name:      "sandbox blocks file_read",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "file_read",
			wantAllow: false,
		},
		{
			name:      "sandbox blocks bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "bash",
			wantAllow: false,
		},
		{
			name:      "sandbox blocks sandbox_shell_exec",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "sandbox_shell_exec",
			wantAllow: false,
		},
		{
			name:      "sandbox blocks sandbox_code_execute",
			mode:      ToolModeCLI,
			preset:    ToolPresetSandbox,
			toolName:  "sandbox_code_execute",
			wantAllow: false,
		},
		{
			name:      "web mode blocks file_read",
			mode:      ToolModeWeb,
			preset:    ToolPresetFull,
			toolName:  "file_read",
			wantAllow: false,
		},
		{
			name:      "web mode allows web_search",
			mode:      ToolModeWeb,
			preset:    ToolPresetFull,
			toolName:  "web_search",
			wantAllow: true,
		},
		{
			name:      "web mode blocks skills",
			mode:      ToolModeWeb,
			preset:    ToolPresetFull,
			toolName:  "skills",
			wantAllow: false,
		},
		{
			name:      "architect allows web_search",
			mode:      ToolModeCLI,
			preset:    ToolPresetArchitect,
			toolName:  "web_search",
			wantAllow: true,
		},
		{
			name:      "architect blocks acp_executor in cli mode",
			mode:      ToolModeCLI,
			preset:    ToolPresetArchitect,
			toolName:  "acp_executor",
			wantAllow: false,
		},
		{
			name:      "architect blocks bash",
			mode:      ToolModeCLI,
			preset:    ToolPresetArchitect,
			toolName:  "bash",
			wantAllow: false,
		},
		{
			name:      "web mode architect allows acp_executor",
			mode:      ToolModeWeb,
			preset:    ToolPresetArchitect,
			toolName:  "acp_executor",
			wantAllow: true,
		},
		{
			name:      "web mode architect blocks file_read",
			mode:      ToolModeWeb,
			preset:    ToolPresetArchitect,
			toolName:  "file_read",
			wantAllow: false,
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
	if len(presets) != 5 {
		t.Errorf("GetAllToolPresets() returned %d presets, want 5", len(presets))
	}

	// Check all expected presets are present
	expected := map[ToolPreset]bool{
		ToolPresetFull:      false,
		ToolPresetReadOnly:  false,
		ToolPresetSafe:      false,
		ToolPresetSandbox:   false,
		ToolPresetArchitect: false,
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
