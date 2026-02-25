package lark

import (
	"path/filepath"
	"testing"
	"time"

	"alex/internal/shared/logging"
)

func TestResolveNoticeStatePathUsesExplicitEnv(t *testing.T) {
	expected := "/tmp/custom-notice-state.json"
	path := resolveNoticeStatePath(func(string) string {
		return expected
	}, func() (string, error) {
		return "/repo", nil
	})
	if path != expected {
		t.Fatalf("resolveNoticeStatePath() = %q, want %q", path, expected)
	}
}

func TestResolveNoticeStatePathFromMainRoot(t *testing.T) {
	path := resolveNoticeStatePath(func(string) string { return "" }, func() (string, error) {
		return "/repo", nil
	})
	expected := filepath.Join("/repo", ".worktrees", "test", "tmp", noticeStateFileName)
	if path != expected {
		t.Fatalf("resolveNoticeStatePath() = %q, want %q", path, expected)
	}
}

func TestResolveNoticeStatePathFromTestWorktree(t *testing.T) {
	path := resolveNoticeStatePath(func(string) string { return "" }, func() (string, error) {
		return filepath.Join("/repo", ".worktrees", "test"), nil
	})
	expected := filepath.Join("/repo", ".worktrees", "test", "tmp", noticeStateFileName)
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
