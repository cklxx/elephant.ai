package config

import (
	"reflect"
	"testing"
)

func TestDefaultRuntimeConfigWatchPaths(t *testing.T) {
	t.Parallel()

	home := "/home/test"
	homeDir := func() (string, error) { return home, nil }

	tests := []struct {
		name      string
		envLookup EnvLookup
		want      []string
	}{
		{
			name: "no ALEX_CONFIG_PATH",
			envLookup: func(string) (string, bool) {
				return "", false
			},
			want: []string{
				"/home/test/.alex/config.yaml",
				"/home/test/.alex/test.yaml",
			},
		},
		{
			name: "ALEX_CONFIG_PATH points to default config",
			envLookup: func(key string) (string, bool) {
				if key == "ALEX_CONFIG_PATH" {
					return "/home/test/.alex/config.yaml", true
				}
				return "", false
			},
			want: []string{
				"/home/test/.alex/config.yaml",
				"/home/test/.alex/test.yaml",
			},
		},
		{
			name: "ALEX_CONFIG_PATH points to test config",
			envLookup: func(key string) (string, bool) {
				if key == "ALEX_CONFIG_PATH" {
					return "/home/test/.alex/test.yaml", true
				}
				return "", false
			},
			want: []string{
				"/home/test/.alex/test.yaml",
				"/home/test/.alex/config.yaml",
			},
		},
		{
			name: "ALEX_CONFIG_PATH points to custom config",
			envLookup: func(key string) (string, bool) {
				if key == "ALEX_CONFIG_PATH" {
					return "/tmp/custom.yaml", true
				}
				return "", false
			},
			want: []string{
				"/tmp/custom.yaml",
				"/home/test/.alex/config.yaml",
				"/home/test/.alex/test.yaml",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DefaultRuntimeConfigWatchPaths(tt.envLookup, homeDir)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("DefaultRuntimeConfigWatchPaths() = %#v; want %#v", got, tt.want)
			}
		})
	}
}

func TestDefaultDotEnvWatchPaths(t *testing.T) {
	t.Parallel()

	cwd := "/repo/project"
	cwdResolver := func() (string, error) { return cwd, nil }

	tests := []struct {
		name      string
		envLookup EnvLookup
		want      []string
	}{
		{
			name: "default .env under cwd",
			envLookup: func(string) (string, bool) {
				return "", false
			},
			want: []string{
				"/repo/project/.env",
			},
		},
		{
			name: "custom relative dotenv path",
			envLookup: func(key string) (string, bool) {
				if key == "ALEX_DOTENV_PATH" {
					return "configs/local.env", true
				}
				return "", false
			},
			want: []string{
				"/repo/project/configs/local.env",
			},
		},
		{
			name: "custom absolute dotenv path",
			envLookup: func(key string) (string, bool) {
				if key == "ALEX_DOTENV_PATH" {
					return "/etc/alex/runtime.env", true
				}
				return "", false
			},
			want: []string{
				"/etc/alex/runtime.env",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := DefaultDotEnvWatchPaths(tt.envLookup, cwdResolver)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("DefaultDotEnvWatchPaths() = %#v; want %#v", got, tt.want)
			}
		})
	}
}
