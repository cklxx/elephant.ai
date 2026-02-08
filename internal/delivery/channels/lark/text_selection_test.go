package lark

import (
	"strings"
	"testing"
)

func TestFormatNumberedOptions(t *testing.T) {
	t.Parallel()
	result := formatNumberedOptions("Which env?", []string{"dev", "staging", "prod"})
	if !strings.Contains(result, "Which env?") {
		t.Fatalf("expected question, got %q", result)
	}
	if !strings.Contains(result, "[1] dev") {
		t.Fatalf("expected [1] dev, got %q", result)
	}
	if !strings.Contains(result, "[2] staging") {
		t.Fatalf("expected [2] staging, got %q", result)
	}
	if !strings.Contains(result, "[3] prod") {
		t.Fatalf("expected [3] prod, got %q", result)
	}
	if !strings.Contains(result, "回复数字选择") {
		t.Fatalf("expected hint, got %q", result)
	}
}

func TestFormatNumberedOptionsSingleOption(t *testing.T) {
	t.Parallel()
	result := formatNumberedOptions("Confirm?", []string{"yes"})
	if !strings.Contains(result, "[1] yes") {
		t.Fatalf("expected [1] yes, got %q", result)
	}
}

func TestParseNumberedReply(t *testing.T) {
	t.Parallel()
	options := []string{"dev", "staging", "prod"}
	tests := []struct {
		input string
		want  string
	}{
		{"1", "dev"},
		{"2", "staging"},
		{"3", "prod"},
		{"0", "0"},
		{"4", "4"},
		{"-1", "-1"},
		{"abc", "abc"},
		{"", ""},
		{"  2  ", "staging"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseNumberedReply(tt.input, options)
			if got != tt.want {
				t.Fatalf("parseNumberedReply(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseNumberedReplyEmptyOptions(t *testing.T) {
	t.Parallel()
	if got := parseNumberedReply("1", nil); got != "1" {
		t.Fatalf("expected raw input for nil options, got %q", got)
	}
	if got := parseNumberedReply("hello", []string{}); got != "hello" {
		t.Fatalf("expected raw input for empty options, got %q", got)
	}
}
