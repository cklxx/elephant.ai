package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func stubLookup(valid map[string]bool) func(string) error {
	return func(term string) error {
		if valid[term] {
			return nil
		}
		return fmt.Errorf("missing terminfo for %s", term)
	}
}

func TestNormalizeTerminalSupported(t *testing.T) {
	lookup := stubLookup(map[string]bool{"wezterm": true})
	got, changed, err := normalizeTerminal("wezterm", "", lookup)
	if err != nil {
		t.Fatalf("normalizeTerminal returned error: %v", err)
	}
	if changed {
		t.Fatalf("expected no change, got changed")
	}
	if got != "wezterm" {
		t.Fatalf("expected wezterm, got %q", got)
	}
}

func TestNormalizeTerminalEmptyFallsBack(t *testing.T) {
	lookup := stubLookup(map[string]bool{defaultTERM: true})
	got, changed, err := normalizeTerminal("", "Apple_Terminal", lookup)
	if err != nil {
		t.Fatalf("normalizeTerminal returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected change for empty TERM")
	}
	if got != defaultTERM {
		t.Fatalf("expected fallback %q, got %q", defaultTERM, got)
	}
}

func TestNormalizeTerminalUnsupportedUsesFallback(t *testing.T) {
	lookup := stubLookup(map[string]bool{"wezterm": true})
	got, changed, err := normalizeTerminal("wezterm-direct", "", lookup)
	if err != nil {
		t.Fatalf("normalizeTerminal returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected change for unsupported TERM")
	}
	if got != "wezterm" {
		t.Fatalf("expected fallback %q, got %q", "wezterm", got)
	}
}

func TestNormalizeTerminalUnsupportedNoFallback(t *testing.T) {
	lookup := stubLookup(map[string]bool{})
	if _, _, err := normalizeTerminal("unknown-term", "", lookup); err == nil {
		t.Fatalf("expected error when no fallback available")
	}
}

func TestNormalizeTerminalUsesTermProgramFallback(t *testing.T) {
	lookup := stubLookup(map[string]bool{"wezterm": true})
	got, changed, err := normalizeTerminal("", "WezTerm", lookup)
	if err != nil {
		t.Fatalf("normalizeTerminal returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected change when deriving from TERM_PROGRAM")
	}
	if got != "wezterm" {
		t.Fatalf("expected fallback %q, got %q", "wezterm", got)
	}
}

func TestNormalizeTerminalFallsBackToTerminfoSuffixes(t *testing.T) {
	lookup := stubLookup(map[string]bool{"xterm-256color": true})
	got, changed, err := normalizeTerminal("screen.xterm-256color", "", lookup)
	if err != nil {
		t.Fatalf("normalizeTerminal returned error: %v", err)
	}
	if !changed {
		t.Fatalf("expected change for namespaced TERM")
	}
	if got != "xterm-256color" {
		t.Fatalf("expected fallback %q, got %q", "xterm-256color", got)
	}
}

func TestPrepareTerminalWithLookupAppliesFallback(t *testing.T) {
	lookup := stubLookup(map[string]bool{defaultTERM: true})
	buf := &bytes.Buffer{}
	var set string
	prepareTerminalWithLookup(
		func(key string) (string, bool) {
			if key == "TERM" {
				return "wezterm-direct", true
			}
			return "", false
		},
		func(key, value string) error {
			if key != "TERM" {
				return fmt.Errorf("unexpected key %q", key)
			}
			set = value
			return nil
		},
		buf,
		lookup,
	)

	if set != defaultTERM {
		t.Fatalf("expected TERM to be set to %q, got %q", defaultTERM, set)
	}
	if !strings.Contains(buf.String(), "Detected unsupported TERM=\"wezterm-direct\"") {
		t.Fatalf("expected warning about fallback, got %q", buf.String())
	}
}
