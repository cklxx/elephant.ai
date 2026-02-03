package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateLegacyUsers_RenamesWhenEmptyTarget(t *testing.T) {
	root := t.TempDir()
	legacyUser := filepath.Join(root, legacyUserDirName, "user-1")
	legacyDaily := filepath.Join(legacyUser, dailyDirName)
	if err := os.MkdirAll(legacyDaily, 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	legacyPath := filepath.Join(legacyDaily, "2026-02-02.md")
	if err := os.WriteFile(legacyPath, []byte("# 2026-02-02\n\nLegacy note"), 0o644); err != nil {
		t.Fatalf("write legacy: %v", err)
	}

	if err := migrateLegacyUsers(root); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	newPath := filepath.Join(root, "user-1", dailyDirName, "2026-02-02.md")
	data, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("expected migrated file, got err: %v", err)
	}
	if !strings.Contains(string(data), "Legacy note") {
		t.Fatalf("expected legacy content preserved, got: %s", string(data))
	}
}

func TestMigrateLegacyUsers_MergesWhenTargetExists(t *testing.T) {
	root := t.TempDir()
	newUser := filepath.Join(root, "user-1")
	newDaily := filepath.Join(newUser, dailyDirName)
	if err := os.MkdirAll(newDaily, 0o755); err != nil {
		t.Fatalf("mkdir new: %v", err)
	}
	newDailyPath := filepath.Join(newDaily, "2026-02-02.md")
	if err := os.WriteFile(newDailyPath, []byte("# 2026-02-02\n\nNew note"), 0o644); err != nil {
		t.Fatalf("write new daily: %v", err)
	}
	newMemoryPath := filepath.Join(newUser, memoryFileName)
	if err := os.WriteFile(newMemoryPath, []byte("# Long-Term Memory\n\nNew fact"), 0o644); err != nil {
		t.Fatalf("write new memory: %v", err)
	}

	legacyUser := filepath.Join(root, legacyUserDirName, "user-1")
	legacyDaily := filepath.Join(legacyUser, dailyDirName)
	if err := os.MkdirAll(legacyDaily, 0o755); err != nil {
		t.Fatalf("mkdir legacy: %v", err)
	}
	legacyDailyPath := filepath.Join(legacyDaily, "2026-02-02.md")
	if err := os.WriteFile(legacyDailyPath, []byte("# 2026-02-02\n\nLegacy daily"), 0o644); err != nil {
		t.Fatalf("write legacy daily: %v", err)
	}
	legacyMemoryPath := filepath.Join(legacyUser, memoryFileName)
	if err := os.WriteFile(legacyMemoryPath, []byte("# Long-Term Memory\n\nLegacy fact"), 0o644); err != nil {
		t.Fatalf("write legacy memory: %v", err)
	}

	if err := migrateLegacyUsers(root); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	mergedDaily, err := os.ReadFile(newDailyPath)
	if err != nil {
		t.Fatalf("read merged daily: %v", err)
	}
	if !strings.Contains(string(mergedDaily), "Legacy Import") || !strings.Contains(string(mergedDaily), "Legacy daily") {
		t.Fatalf("expected legacy daily merged, got: %s", string(mergedDaily))
	}
	mergedMemory, err := os.ReadFile(newMemoryPath)
	if err != nil {
		t.Fatalf("read merged memory: %v", err)
	}
	if !strings.Contains(string(mergedMemory), "Legacy Import") || !strings.Contains(string(mergedMemory), "Legacy fact") {
		t.Fatalf("expected legacy memory merged, got: %s", string(mergedMemory))
	}
}
