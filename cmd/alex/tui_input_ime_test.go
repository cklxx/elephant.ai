package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestApplyIMEKeyInsertsRunes(t *testing.T) {
	buffer, cursorPos, handled := applyIMEKey(nil, 0, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("你好")})
	if !handled {
		t.Fatal("expected key to be handled")
	}
	if got := string(buffer); got != "你好" {
		t.Fatalf("expected buffer to be 你好, got %q", got)
	}
	if cursorPos != 2 {
		t.Fatalf("expected cursor at position 2, got %d", cursorPos)
	}
}

func TestApplyIMEKeyBackspace(t *testing.T) {
	buffer := []rune("你好")
	buffer, cursorPos, handled := applyIMEKey(buffer, len(buffer), tea.KeyMsg{Type: tea.KeyBackspace})
	if !handled {
		t.Fatal("expected backspace to be handled")
	}
	if got := string(buffer); got != "你" {
		t.Fatalf("expected buffer to be 你, got %q", got)
	}
	if cursorPos != 1 {
		t.Fatalf("expected cursor at position 1, got %d", cursorPos)
	}
}

func TestApplyIMEKeyBackspaceGrapheme(t *testing.T) {
	buffer := []rune("e\u0301")
	buffer, cursorPos, handled := applyIMEKey(buffer, len(buffer), tea.KeyMsg{Type: tea.KeyBackspace})
	if !handled {
		t.Fatal("expected backspace to be handled")
	}
	if got := string(buffer); got != "" {
		t.Fatalf("expected buffer to be empty, got %q", got)
	}
	if cursorPos != 0 {
		t.Fatalf("expected cursor at position 0, got %d", cursorPos)
	}
}

func TestApplyIMEKeyBackspaceRune(t *testing.T) {
	buffer := []rune("你好")
	buffer, cursorPos, handled := applyIMEKey(buffer, len(buffer), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{127}})
	if !handled {
		t.Fatal("expected rune backspace to be handled")
	}
	if got := string(buffer); got != "你" {
		t.Fatalf("expected buffer to be 你, got %q", got)
	}
	if cursorPos != 1 {
		t.Fatalf("expected cursor at position 1, got %d", cursorPos)
	}

	buffer = []rune("你好")
	buffer, cursorPos, handled = applyIMEKey(buffer, len(buffer), tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{8}})
	if !handled {
		t.Fatal("expected ctrl+h rune to be handled")
	}
	if got := string(buffer); got != "你" {
		t.Fatalf("expected buffer to be 你, got %q", got)
	}
	if cursorPos != 1 {
		t.Fatalf("expected cursor at position 1, got %d", cursorPos)
	}
}

func TestApplyIMEKeyUnrelatedKey(t *testing.T) {
	buffer := []rune("hello")
	updated, cursorPos, handled := applyIMEKey(buffer, 5, tea.KeyMsg{Type: tea.KeyEnter})
	if handled {
		t.Fatal("expected enter not to be handled")
	}
	if got := string(updated); got != "hello" {
		t.Fatalf("expected buffer unchanged, got %q", got)
	}
	if cursorPos != 5 {
		t.Fatalf("expected cursor unchanged at position 5, got %d", cursorPos)
	}
}

func TestApplyIMEKeyBackspaceInMiddle(t *testing.T) {
	// Test deleting in the middle: "你好世界" with cursor after "好"
	buffer := []rune("你好世界")
	// Cursor is at position 2 (after "你好")
	buffer, cursorPos, handled := applyIMEKey(buffer, 2, tea.KeyMsg{Type: tea.KeyBackspace})
	if !handled {
		t.Fatal("expected backspace to be handled")
	}
	if got := string(buffer); got != "你世界" {
		t.Fatalf("expected buffer to be 你世界, got %q", got)
	}
	if cursorPos != 1 {
		t.Fatalf("expected cursor at position 1, got %d", cursorPos)
	}
}

func TestApplyIMEKeyInsertInMiddle(t *testing.T) {
	// Test inserting in the middle: "你界" with cursor after "你"
	buffer := []rune("你界")
	// Insert "好世" at position 1
	buffer, cursorPos, handled := applyIMEKey(buffer, 1, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("好世")})
	if !handled {
		t.Fatal("expected runes to be handled")
	}
	if got := string(buffer); got != "你好世界" {
		t.Fatalf("expected buffer to be 你好世界, got %q", got)
	}
	if cursorPos != 3 {
		t.Fatalf("expected cursor at position 3, got %d", cursorPos)
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
