package external

import (
	"slices"
	"testing"
	"time"

	"alex/internal/shared/config"
)

func TestPickFirstNonEmpty(t *testing.T) {
	tests := []struct {
		values []string
		want   string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"", "", "c"}, "c"},
		{[]string{"a", "b"}, "a"},
		{[]string{"  ", "b"}, "b"},
		{[]string{" a "}, "a"},
	}
	for _, tt := range tests {
		got := pickFirstNonEmpty(tt.values...)
		if got != tt.want {
			t.Errorf("pickFirstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
		}
	}
}

func TestRequestKey(t *testing.T) {
	got := requestKey("task-1", "req-1")
	if got != "task-1:req-1" {
		t.Errorf("expected task-1:req-1, got %s", got)
	}
}

func TestRequestKey_WithSpaces(t *testing.T) {
	got := requestKey("  task  ", "  req  ")
	if got != "task:req" {
		t.Errorf("expected task:req, got %s", got)
	}
}

func TestNewRegistry_RegistersConfiguredAgentsAndGenericCLI(t *testing.T) {
	cfg := config.ExternalAgentsConfig{
		ClaudeCode: config.ClaudeCodeConfig{
			Enabled: true,
			Env:     map[string]string{"ANTHROPIC_API_KEY": "claude-key"},
		},
		Codex: config.CLIAgentConfig{
			Enabled: true,
			Env:     map[string]string{"OPENAI_API_KEY": "codex-key"},
		},
		Kimi: config.CLIAgentConfig{
			Enabled: true,
			Env:     map[string]string{"KIMI_API_KEY": "kimi-key"},
		},
	}

	registry := NewRegistry(cfg, nil, nil)
	got := registry.SupportedTypes()
	slices.Sort(got)
	want := []string{"claude_code", "codex", "generic_cli", "kimi"}
	if !slices.Equal(got, want) {
		t.Fatalf("SupportedTypes() = %v, want %v", got, want)
	}
}

func TestPickGenericTimeout_PrefersConfiguredAgentTimeouts(t *testing.T) {
	cases := []struct {
		name string
		cfg  config.ExternalAgentsConfig
		want time.Duration
	}{
		{
			name: "prefers codex",
			cfg: config.ExternalAgentsConfig{
				Codex:      config.CLIAgentConfig{Timeout: 12 * time.Minute},
				Kimi:       config.CLIAgentConfig{Timeout: 8 * time.Minute},
				ClaudeCode: config.ClaudeCodeConfig{Timeout: 5 * time.Minute},
			},
			want: 12 * time.Minute,
		},
		{
			name: "falls back to kimi",
			cfg: config.ExternalAgentsConfig{
				Kimi:       config.CLIAgentConfig{Timeout: 8 * time.Minute},
				ClaudeCode: config.ClaudeCodeConfig{Timeout: 5 * time.Minute},
			},
			want: 8 * time.Minute,
		},
		{
			name: "falls back to claude",
			cfg: config.ExternalAgentsConfig{
				ClaudeCode: config.ClaudeCodeConfig{Timeout: 5 * time.Minute},
			},
			want: 5 * time.Minute,
		},
		{
			name: "uses default",
			cfg:  config.ExternalAgentsConfig{},
			want: 30 * time.Minute,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := pickGenericTimeout(tc.cfg); got != tc.want {
				t.Fatalf("pickGenericTimeout() = %s, want %s", got, tc.want)
			}
		})
	}
}
