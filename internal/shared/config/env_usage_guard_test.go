package config_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoUnapprovedGetenv ensures new os.Getenv usages are surfaced during tests.
func TestNoUnapprovedGetenv(t *testing.T) {
	moduleRoot := findModuleRoot(t)

	allowed := newStringSet(t,
		"cmd/auth-user-seed/main.go",
		"cmd/alex-server/main.go",
		"internal/infra/external/bridge/executor_integration_test.go",
		"internal/infra/external/claudecode/executor_sdk_integration_test.go",
		"internal/infra/external/claudecode/executor_sdk_integration_tool_test.go",
		"internal/infra/kernel/postgres_store_test.go",
		"internal/shared/utils/logger.go",
	)

	skipDirs := map[string]struct{}{
		".cache":       {},
		".elephant":    {},
		".git":         {},
		".toolchains":  {},
		".worktrees":   {},
		"elephant.ai.worktrees": {},  // Additional worktrees directory
		"logs":         {},
		"node_modules": {},
		"vendor":       {},
	}

	var violations []string

	err := filepath.WalkDir(moduleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".worktrees-") {
				return filepath.SkipDir
			}
			if _, ok := skipDirs[d.Name()]; ok {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(moduleRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "internal/shared/config/env_usage_guard_test.go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(data, []byte("os.Getenv(")) {
			return nil
		}
		if _, ok := allowed[rel]; ok {
			return nil
		}
		violations = append(violations, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk module: %v", err)
	}

	if len(violations) > 0 {
		t.Fatalf("os.Getenv usage is restricted to config-managed code; add allowlist entry or migrate to internal/shared/config loader: %s", strings.Join(violations, ", "))
	}
}

func findModuleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

func newStringSet(t *testing.T, entries ...string) map[string]struct{} {
	t.Helper()

	set := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if _, exists := set[entry]; exists {
			t.Fatalf("duplicate allowlist entry: %s", entry)
		}
		set[entry] = struct{}{}
	}
	return set
}
