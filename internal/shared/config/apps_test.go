package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAndLoadAppsConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	input := AppsConfig{
		Plugins: []AppPluginConfig{
			{ID: "lark", Name: "Lark"},
		},
	}

	savedPath, err := SaveAppsConfig(input, WithConfigPath(path))
	if err != nil {
		t.Fatalf("SaveAppsConfig error: %v", err)
	}
	if savedPath != path {
		t.Fatalf("expected path %q, got %q", path, savedPath)
	}

	loaded, loadedPath, err := LoadAppsConfig(WithConfigPath(path))
	if err != nil {
		t.Fatalf("LoadAppsConfig error: %v", err)
	}
	if loadedPath != path {
		t.Fatalf("expected path %q, got %q", path, loadedPath)
	}
	if len(loaded.Plugins) != 1 || loaded.Plugins[0].ID != "lark" {
		t.Fatalf("unexpected loaded plugins: %#v", loaded.Plugins)
	}
}

func TestSaveAppsConfigClearsSectionWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	if _, err := SaveAppsConfig(AppsConfig{}, WithConfigPath(path)); err != nil {
		t.Fatalf("SaveAppsConfig error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "apps:") {
		t.Fatalf("expected apps section to be removed, got:\n%s", string(data))
	}
}
