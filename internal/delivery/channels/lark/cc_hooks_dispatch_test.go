package lark

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/delivery/channels"
	"alex/internal/shared/logging"
)

func TestWriteCCHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")

	err := writeCCHooks(path, "http://localhost:8080", "tok123")
	if err != nil {
		t.Fatalf("writeCCHooks: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "ELEPHANT_HOOKS_URL=http://localhost:8080") {
		t.Errorf("expected server_url in command, got: %s", content)
	}
	if !strings.Contains(content, "ELEPHANT_HOOKS_TOKEN=tok123") {
		t.Errorf("expected token in command, got: %s", content)
	}
	if !strings.Contains(content, "PostToolUse") {
		t.Errorf("expected PostToolUse hook, got: %s", content)
	}
	if !strings.Contains(content, "Stop") {
		t.Errorf("expected Stop hook, got: %s", content)
	}
}

func TestWriteCCHooksNoToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")

	err := writeCCHooks(path, "http://localhost:9090", "")
	if err != nil {
		t.Fatalf("writeCCHooks: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "ELEPHANT_HOOKS_URL=http://localhost:9090") {
		t.Errorf("expected server_url, got: %s", content)
	}
	if strings.Contains(content, "ELEPHANT_HOOKS_TOKEN") {
		t.Errorf("expected no token in command, got: %s", content)
	}
}

func TestWriteCCHooksMergesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")

	// Pre-populate with existing settings.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"permissions":{"allow":["Read"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeCCHooks(path, "http://localhost:8080", "")
	if err != nil {
		t.Fatalf("writeCCHooks: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if _, ok := result["permissions"]; !ok {
		t.Error("expected existing permissions to be preserved")
	}
	if _, ok := result["hooks"]; !ok {
		t.Error("expected hooks to be added")
	}
}

func TestRemoveCCHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")

	// Write hooks first.
	if err := writeCCHooks(path, "http://localhost:8080", ""); err != nil {
		t.Fatal(err)
	}

	// Remove.
	if err := removeCCHooks(path); err != nil {
		t.Fatalf("removeCCHooks: %v", err)
	}

	// File should be deleted since hooks was the only key.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected settings file to be removed when empty")
	}
}

func TestRemoveCCHooksPreservesOtherKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")

	// Write with existing settings.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"hooks":{"PostToolUse":[]},"permissions":{"allow":["Read"]}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := removeCCHooks(path); err != nil {
		t.Fatalf("removeCCHooks: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := result["hooks"]; ok {
		t.Error("expected hooks to be removed")
	}
	if _, ok := result["permissions"]; !ok {
		t.Error("expected permissions to be preserved")
	}
}

func TestRemoveCCHooksFileNotExist(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".claude", "settings.local.json")
	if err := removeCCHooks(path); err != nil {
		t.Fatalf("expected no error for non-existent file, got: %v", err)
	}
}

func TestRunCCHooksSetup(t *testing.T) {
	dir := t.TempDir()
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig:   channels.BaseConfig{SessionPrefix: "lark", AllowGroups: true},
			AppID:        "test",
			AppSecret:    "secret",
			WorkspaceDir: dir,
			CCHooksAutoConfig: &CCHooksAutoConfig{
				ServerURL: "http://localhost:8080",
				Token:     "tok123",
			},
		},
		logger:    logging.OrNop(nil),
		messenger: recorder,
	}

	msg := &incomingMessage{
		chatID:    "oc_hooks_chat",
		messageID: "om_hooks_msg",
		senderID:  "ou_hooks_sender",
		content:   "/notice",
		isGroup:   true,
	}

	gw.runCCHooksSetup(msg)

	// Verify file was written.
	settingsPath := filepath.Join(dir, ".claude", "settings.local.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("settings file not created: %v", err)
	}
	if !strings.Contains(string(data), "tok123") {
		t.Errorf("expected token in settings, got: %s", data)
	}

	// Verify reply was sent.
	calls := recorder.CallsByMethod("SendMessage")
	if len(calls) == 0 {
		t.Fatal("expected a reply message to be sent")
	}
	replyText := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(replyText, "配置完成") {
		t.Errorf("unexpected reply: %q", replyText)
	}
}

func TestRunCCHooksSetupNilConfig(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{SessionPrefix: "lark"},
			AppID:      "test",
			AppSecret:  "secret",
		},
		logger:    logging.OrNop(nil),
		messenger: recorder,
	}

	msg := &incomingMessage{
		chatID:    "oc_chat",
		messageID: "om_msg",
		senderID:  "ou_sender",
		isGroup:   true,
	}

	gw.runCCHooksSetup(msg)

	calls := recorder.CallsByMethod("SendMessage")
	if len(calls) != 0 {
		t.Fatalf("expected no messages when CCHooksAutoConfig is nil, got %d", len(calls))
	}
}

func TestBuildHookCommand(t *testing.T) {
	tests := []struct {
		serverURL, token string
		wantContains     []string
		wantNotContains  []string
	}{
		{
			serverURL:    "http://localhost:8080",
			token:        "tok",
			wantContains: []string{"ELEPHANT_HOOKS_URL=http://localhost:8080", "ELEPHANT_HOOKS_TOKEN=tok", "notify_lark.sh"},
		},
		{
			serverURL:       "http://example.com",
			token:           "",
			wantContains:    []string{"ELEPHANT_HOOKS_URL=http://example.com", "notify_lark.sh"},
			wantNotContains: []string{"ELEPHANT_HOOKS_TOKEN"},
		},
	}

	for _, tt := range tests {
		result := buildHookCommand(tt.serverURL, tt.token)
		for _, want := range tt.wantContains {
			if !strings.Contains(result, want) {
				t.Errorf("buildHookCommand(%q, %q) = %q, expected to contain %q", tt.serverURL, tt.token, result, want)
			}
		}
		for _, notWant := range tt.wantNotContains {
			if strings.Contains(result, notWant) {
				t.Errorf("buildHookCommand(%q, %q) = %q, expected NOT to contain %q", tt.serverURL, tt.token, result, notWant)
			}
		}
	}
}
