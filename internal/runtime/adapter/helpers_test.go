package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// hasBashPrompt
// ---------------------------------------------------------------------------

func TestHasBashPrompt(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{"bash dollar sign", "user@host:~$", true},
		{"bash dollar space", "user@host:~$ ", true},
		{"zsh percent", "user@host:~%", true},
		{"zsh percent space", "user@host:~% ", true},
		{"trailing newlines ignored", "user@host:~$\n\n\n", true},
		{"blank lines before prompt", "\n\nsome output\nuser@host:~$\n", true},
		{"CC still running", "❯ thinking...\n", false},
		{"empty output", "", false},
		{"only blank lines", "\n\n\n", false},
		{"output with no prompt", "Hello, how can I help?\n", false},
		{"prompt mid line not last", "user@host:~$ \nCC output here", false},
		{"bare dollar", "$", true},
		{"bare percent", "%", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, hasBashPrompt(tc.output))
		})
	}
}

// ---------------------------------------------------------------------------
// alreadyRegistered
// ---------------------------------------------------------------------------

func TestAlreadyRegistered(t *testing.T) {
	scriptPath := "/path/to/notify_runtime.sh"

	makeEntry := func(cmd string) map[string]any {
		return map[string]any{
			"hooks": []any{
				map[string]any{
					"type":    "command",
					"command": cmd,
					"async":   true,
				},
			},
		}
	}

	tests := []struct {
		name   string
		hooks  map[string]any
		event  string
		want   bool
	}{
		{
			"event key missing",
			map[string]any{},
			"PostToolUse",
			false,
		},
		{
			"event value not array",
			map[string]any{"PostToolUse": "not-an-array"},
			"PostToolUse",
			false,
		},
		{
			"entry not a map",
			map[string]any{"PostToolUse": []any{"string-entry"}},
			"PostToolUse",
			false,
		},
		{
			"hooks key not array",
			map[string]any{"PostToolUse": []any{map[string]any{"hooks": "not-array"}}},
			"PostToolUse",
			false,
		},
		{
			"hook entry not a map",
			map[string]any{"PostToolUse": []any{map[string]any{"hooks": []any{"string"}}}},
			"PostToolUse",
			false,
		},
		{
			"command matches",
			map[string]any{"PostToolUse": []any{makeEntry(scriptPath)}},
			"PostToolUse",
			true,
		},
		{
			"command does not match",
			map[string]any{"PostToolUse": []any{makeEntry("/other/script.sh")}},
			"PostToolUse",
			false,
		},
		{
			"multiple entries one matches",
			map[string]any{
				"PostToolUse": []any{
					makeEntry("/other/script.sh"),
					makeEntry(scriptPath),
				},
			},
			"PostToolUse",
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, alreadyRegistered(tc.hooks, tc.event, scriptPath))
		})
	}
}

// ---------------------------------------------------------------------------
// ensureCCHooks — filesystem integration tests using temp dirs
// ---------------------------------------------------------------------------

func TestEnsureCCHooks_CreatesSettingsFromScratch(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	scriptPath := "/fake/notify_runtime.sh"
	ensureCCHooks(scriptPath)

	settingsPath := filepath.Join(tmpHome, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "hooks key should exist")

	for _, event := range []string{"PostToolUse", "Stop"} {
		entries, ok := hooks[event].([]any)
		require.True(t, ok, "event %s should be an array", event)
		assert.Len(t, entries, 1)
	}
}

func TestEnsureCCHooks_IdempotentSecondCall(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	scriptPath := "/fake/notify_runtime.sh"
	ensureCCHooks(scriptPath)
	ensureCCHooks(scriptPath) // second call should be no-op

	settingsPath := filepath.Join(tmpHome, ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))

	hooks := settings["hooks"].(map[string]any)
	for _, event := range []string{"PostToolUse", "Stop"} {
		entries := hooks[event].([]any)
		assert.Len(t, entries, 1, "should not duplicate entries for %s", event)
	}
}

func TestEnsureCCHooks_PreservesExistingSettings(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	claudeDir := filepath.Join(tmpHome, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o700))

	existing := map[string]any{
		"customKey": "preserved",
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), data, 0o644))

	ensureCCHooks("/fake/script.sh")

	result, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
	require.NoError(t, err)

	var settings map[string]any
	require.NoError(t, json.Unmarshal(result, &settings))

	assert.Equal(t, "preserved", settings["customKey"])
	assert.NotNil(t, settings["hooks"])
}

func TestEnsureCCHooks_MalformedJSON(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	claudeDir := filepath.Join(tmpHome, ".claude")
	require.NoError(t, os.MkdirAll(claudeDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(claudeDir, "settings.json"), []byte("{bad json}"), 0o644))

	// Should not panic, just log and return.
	ensureCCHooks("/fake/script.sh")
}

// ---------------------------------------------------------------------------
// shellQuote
// ---------------------------------------------------------------------------

func TestShellQuote(t *testing.T) {
	assert.Equal(t, "'hello'", shellQuote("hello"))
	assert.Equal(t, "'it'\\''s'", shellQuote("it's"))
	assert.Equal(t, "''", shellQuote(""))
}

// ---------------------------------------------------------------------------
// findNotifyScript
// ---------------------------------------------------------------------------

func TestFindNotifyScript_Found(t *testing.T) {
	tmpDir := t.TempDir()
	scriptDir := filepath.Join(tmpDir, "scripts", "cc_hooks")
	require.NoError(t, os.MkdirAll(scriptDir, 0o755))

	scriptPath := filepath.Join(scriptDir, "notify_runtime.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0o755))

	result := findNotifyScript(tmpDir)
	assert.Equal(t, scriptPath, result)
}

func TestFindNotifyScript_FoundInParent(t *testing.T) {
	tmpDir := t.TempDir()
	scriptDir := filepath.Join(tmpDir, "scripts", "cc_hooks")
	require.NoError(t, os.MkdirAll(scriptDir, 0o755))

	scriptPath := filepath.Join(scriptDir, "notify_runtime.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh"), 0o755))

	child := filepath.Join(tmpDir, "subdir", "deep")
	require.NoError(t, os.MkdirAll(child, 0o755))

	result := findNotifyScript(child)
	assert.Equal(t, scriptPath, result)
}

func TestFindNotifyScript_NotFound(t *testing.T) {
	result := findNotifyScript(t.TempDir())
	assert.Equal(t, "", result)
}
