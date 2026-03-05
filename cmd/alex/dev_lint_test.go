package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWebLintNeedsInstallWhenEslintMissing(t *testing.T) {
	t.Parallel()

	needsInstall, err := webLintNeedsInstall(t.TempDir())
	if err != nil {
		t.Fatalf("webLintNeedsInstall() error = %v", err)
	}
	if !needsInstall {
		t.Fatal("webLintNeedsInstall() = false, want true when eslint is missing")
	}
}

func TestWebLintNeedsInstallWhenEslintExists(t *testing.T) {
	t.Parallel()

	webDir := t.TempDir()
	eslintPath := filepath.Join(webDir, "node_modules", ".bin", "eslint")
	if err := os.MkdirAll(filepath.Dir(eslintPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(eslintPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	needsInstall, err := webLintNeedsInstall(webDir)
	if err != nil {
		t.Fatalf("webLintNeedsInstall() error = %v", err)
	}
	if needsInstall {
		t.Fatal("webLintNeedsInstall() = true, want false when eslint exists")
	}
}

func TestWebLintNeedsInstallStatFailure(t *testing.T) {
	t.Parallel()

	webDir := t.TempDir()
	nodeModulesPath := filepath.Join(webDir, "node_modules")
	if err := os.WriteFile(nodeModulesPath, []byte("not-a-dir"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := webLintNeedsInstall(webDir)
	if err == nil {
		t.Fatal("webLintNeedsInstall() expected error for invalid node_modules path")
	}
}
