package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRuntimeDirsCreatesDirectories(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	pidDir := filepath.Join(root, "runtime", "pids")
	logDir := filepath.Join(root, "runtime", "logs")

	if err := EnsureRuntimeDirs(pidDir, logDir); err != nil {
		t.Fatalf("ensure runtime dirs: %v", err)
	}

	if info, err := os.Stat(pidDir); err != nil || !info.IsDir() {
		t.Fatalf("pid dir not created: info=%v err=%v", info, err)
	}
	if info, err := os.Stat(logDir); err != nil || !info.IsDir() {
		t.Fatalf("log dir not created: info=%v err=%v", info, err)
	}
}
