package main

import (
	"path/filepath"
	"testing"

	"alex/internal/devops"
)

func TestReadConfiguredAlexConfigPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
		want string
	}{
		{
			name: "missing env returns empty",
			env:  nil,
			want: "",
		},
		{
			name: "trimmed env path is returned",
			env:  map[string]string{"ALEX_CONFIG_PATH": "  /tmp/alex.yaml  "},
			want: "/tmp/alex.yaml",
		},
		{
			name: "blank env returns empty",
			env:  map[string]string{"ALEX_CONFIG_PATH": "   "},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := readConfiguredAlexConfigPath(func(key string) (string, bool) {
				v, ok := tt.env[key]
				return v, ok
			})
			if got != tt.want {
				t.Fatalf("readConfiguredAlexConfigPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCoreServiceURLs(t *testing.T) {
	t.Parallel()

	cfg := &devops.DevConfig{ServerPort: 19090, WebPort: 13000}
	got := coreServiceURLs(cfg)

	if got["backend"] != "http://localhost:19090" {
		t.Fatalf("backend URL = %q", got["backend"])
	}
	if got["web"] != "http://localhost:13000" {
		t.Fatalf("web URL = %q", got["web"])
	}
	if len(got) != 2 {
		t.Fatalf("unexpected URL map size: %d", len(got))
	}
}

func TestResolveSharedDevPIDDirUsesConfiguredPathDirectory(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config", "alex.yaml")
	t.Setenv("LARK_PID_DIR", "")
	t.Setenv("ALEX_CONFIG_PATH", "  "+configPath+"  ")

	got, err := resolveSharedDevPIDDir(tmp)
	if err != nil {
		t.Fatalf("resolveSharedDevPIDDir returned error: %v", err)
	}
	want := filepath.Join(filepath.Dir(configPath), "pids")
	if got != want {
		t.Fatalf("resolveSharedDevPIDDir() = %q, want %q", got, want)
	}
}
