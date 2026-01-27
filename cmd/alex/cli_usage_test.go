package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigUsageLineWithExplicitPath(t *testing.T) {
	envLookup := func(key string) (string, bool) {
		if key == "ALEX_CONFIG_PATH" {
			return "/tmp/custom.yaml", true
		}
		return "", false
	}
	line := configUsageLineWith(envLookup, func() (string, error) {
		return "/home/ignored", nil
	})
	if !strings.Contains(line, "/tmp/custom.yaml") {
		t.Fatalf("expected explicit path to be used, got %q", line)
	}
	if !strings.Contains(line, "ALEX_CONFIG_PATH") {
		t.Fatalf("expected source label to mention ALEX_CONFIG_PATH, got %q", line)
	}
}

func TestConfigUsageLineWithHomeDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	envLookup := func(string) (string, bool) {
		return "", false
	}
	line := configUsageLineWith(envLookup, func() (string, error) {
		return home, nil
	})
	expected := "  Config file: " + filepath.Join(home, ".alex", "config.yaml")
	if line != expected {
		t.Fatalf("expected %q, got %q", expected, line)
	}
}
