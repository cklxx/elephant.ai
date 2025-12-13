package bootstrap

import (
	"reflect"
	"testing"
)

func TestParseAllowedOrigins(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty",
			input: "",
			want:  []string{},
		},
		{
			name:  "mixed separators and duplicates",
			input: " https://a.example.com, https://b.example.com; https://a.example.com\nhttp://c.example.com\t",
			want:  []string{"https://a.example.com", "https://b.example.com", "http://c.example.com"},
		},
		{
			name:  "trims whitespace",
			input: "  http://localhost:3000  ,   http://localhost:3001 ",
			want:  []string{"http://localhost:3000", "http://localhost:3001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAllowedOrigins(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseAllowedOrigins(%q) = %#v; want %#v", tt.input, got, tt.want)
			}
		})
	}
}
