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
