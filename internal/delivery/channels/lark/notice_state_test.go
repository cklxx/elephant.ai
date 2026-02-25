package lark

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/shared/logging"
)

func TestResolveNoticeStatePathUsesExplicitEnv(t *testing.T) {
	expected := "/tmp/custom-notice-state.json"
	path := resolveNoticeStatePath(func(key string) string {
		if key == noticeStatePathEnv {
			return expected
		}
		return ""
	}, func() (string, error) {
		return "/repo", nil
	})
	if path != expected {
		t.Fatalf("resolveNoticeStatePath() = %q, want %q", path, expected)
	}
}

func TestResolveNoticeStatePathFromConfigPath(t *testing.T) {
	path := resolveNoticeStatePath(func(key string) string {
		if key == "ALEX_CONFIG_PATH" {
			return "/repo/custom-config.yaml"
		}
		return ""
	}, func() (string, error) {
		return "/repo", nil
	})
	expected := filepath.Join("/repo", noticeStateFileName)
	if path != expected {
		t.Fatalf("resolveNoticeStatePath() = %q, want %q", path, expected)
	}
}

func TestResolveNoticeStatePathFromDefaultHomeConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := resolveNoticeStatePath(func(string) string { return "" }, nil)
	expected := filepath.Join(home, ".alex", noticeStateFileName)
	if path != expected {
		t.Fatalf("resolveNoticeStatePath() = %q, want %q", path, expected)
	}
}

func TestNoticeStateStoreSaveLoadClear(t *testing.T) {
	tmp := t.TempDir()
	store := &noticeStateStore{
		path:   filepath.Join(tmp, "lark-notice.state.json"),
		logger: logging.OrNop(nil),
		now: func() time.Time {
			return time.Date(2026, 2, 9, 8, 0, 0, 0, time.UTC)
		},
	}

	binding, err := store.Save("oc_notice", "ou_user", "")
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if binding.ChatID != "oc_notice" {
		t.Fatalf("Save() chat_id = %q, want oc_notice", binding.ChatID)
	}

	loaded, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok {
		t.Fatal("Load() ok = false, want true")
	}
	if loaded.ChatID != "oc_notice" {
		t.Fatalf("Load() chat_id = %q, want oc_notice", loaded.ChatID)
	}
	if loaded.SetByUserID != "ou_user" {
		t.Fatalf("Load() set_by_user_id = %q, want ou_user", loaded.SetByUserID)
	}
	if loaded.SetAt == "" || loaded.UpdatedAt == "" {
		t.Fatalf("Load() timestamps are empty: %+v", loaded)
	}

	if err := store.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	_, ok, err = store.Load()
	if err != nil {
		t.Fatalf("Load() after clear error = %v", err)
	}
	if ok {
		t.Fatal("Load() after clear ok = true, want false")
	}
}

func TestNewNoticeStateLoaderReadsBoundChatID(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "custom-notice-state.json")
	t.Setenv(noticeStatePathEnv, statePath)
	t.Setenv("ALEX_CONFIG_PATH", filepath.Join(tmp, "config.yaml"))

	payload := `{"chat_id":"oc_test_notice","set_at":"2026-02-25T10:02:11Z","updated_at":"2026-02-25T10:02:11Z"}`
	if err := os.WriteFile(statePath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write notice state: %v", err)
	}

	loader := NewNoticeStateLoader(logging.OrNop(nil))
	chatID, ok, err := loader()
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !ok {
		t.Fatal("loader() ok = false, want true")
	}
	if chatID != "oc_test_notice" {
		t.Fatalf("loader() chat_id = %q, want %q", chatID, "oc_test_notice")
	}
}
