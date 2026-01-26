package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestApplyIMEKeyInsertsRunes(t *testing.T) {
	buffer, handled := applyIMEKey(nil, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("你好")})
	if !handled {
		t.Fatal("expected key to be handled")
	}
	if got := string(buffer); got != "你好" {
		t.Fatalf("expected buffer to be 你好, got %q", got)
	}
}

func TestApplyIMEKeyBackspace(t *testing.T) {
	buffer := []rune("你好")
	buffer, handled := applyIMEKey(buffer, tea.KeyMsg{Type: tea.KeyBackspace})
	if !handled {
		t.Fatal("expected backspace to be handled")
	}
	if got := string(buffer); got != "你" {
		t.Fatalf("expected buffer to be 你, got %q", got)
	}
}

func TestApplyIMEKeyUnrelatedKey(t *testing.T) {
	buffer := []rune("hello")
	updated, handled := applyIMEKey(buffer, tea.KeyMsg{Type: tea.KeyEnter})
	if handled {
		t.Fatal("expected enter not to be handled")
	}
	if got := string(updated); got != "hello" {
		t.Fatalf("expected buffer unchanged, got %q", got)
	}
}

func TestShouldUseIMEInput(t *testing.T) {
	lookup := func(values map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			value, ok := values[key]
			return value, ok
		}
	}

	cases := []struct {
		name   string
		env    map[string]string
		expect bool
	}{
		{
			name:   "explicit ime mode",
			env:    map[string]string{"ALEX_TUI_INPUT": "ime"},
			expect: true,
		},
		{
			name:   "raw overrides ime",
			env:    map[string]string{"ALEX_TUI_INPUT": "raw", "ALEX_TUI_IME": "1"},
			expect: false,
		},
		{
			name:   "ime flag",
			env:    map[string]string{"ALEX_TUI_IME": "true"},
			expect: true,
		},
		{
			name:   "cjk locale",
			env:    map[string]string{"LANG": "zh_CN.UTF-8"},
			expect: true,
		},
		{
			name:   "default off",
			env:    map[string]string{},
			expect: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldUseIMEInput(lookup(tc.env)); got != tc.expect {
				t.Fatalf("expected %v, got %v", tc.expect, got)
			}
		})
	}
}
