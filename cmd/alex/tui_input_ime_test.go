package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestIsBackspaceKey(t *testing.T) {
	cases := []struct {
		name   string
		msg    tea.KeyMsg
		expect bool
	}{
		{
			name:   "KeyBackspace type",
			msg:    tea.KeyMsg{Type: tea.KeyBackspace},
			expect: true,
		},
		{
			name:   "KeyDelete type",
			msg:    tea.KeyMsg{Type: tea.KeyDelete},
			expect: true,
		},
		{
			name:   "backspace string",
			msg:    tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{127}},
			expect: true,
		},
		{
			name:   "ctrl+h (rune 8)",
			msg:    tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{8}},
			expect: true,
		},
		{
			name:   "enter not backspace",
			msg:    tea.KeyMsg{Type: tea.KeyEnter},
			expect: false,
		},
		{
			name:   "normal runes not backspace",
			msg:    tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("你好")},
			expect: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBackspaceKey(tc.msg); got != tc.expect {
				t.Fatalf("expected %v, got %v", tc.expect, got)
			}
		})
	}
}

func TestApplyGraphemeBackspace(t *testing.T) {
	cases := []struct {
		name         string
		value        string
		cursorPos    int
		expectValue  string
		expectCursor int
	}{
		{
			name:         "delete last Chinese character",
			value:        "你好",
			cursorPos:    2,
			expectValue:  "你",
			expectCursor: 1,
		},
		{
			name:         "delete middle Chinese character",
			value:        "你好世界",
			cursorPos:    2,
			expectValue:  "你世界",
			expectCursor: 1,
		},
		{
			name:         "delete first Chinese character",
			value:        "你好",
			cursorPos:    1,
			expectValue:  "好",
			expectCursor: 0,
		},
		{
			name:         "empty string",
			value:        "",
			cursorPos:    0,
			expectValue:  "",
			expectCursor: 0,
		},
		{
			name:         "cursor at start",
			value:        "你好",
			cursorPos:    0,
			expectValue:  "你好",
			expectCursor: 0,
		},
		{
			name:         "combining character (e + accent)",
			value:        "e\u0301", // é as combining sequence
			cursorPos:    2,
			expectValue:  "",
			expectCursor: 0,
		},
		{
			name:         "mixed ASCII and Chinese",
			value:        "hello你好",
			cursorPos:    7,
			expectValue:  "hello你",
			expectCursor: 6,
		},
		{
			name:         "delete ASCII character",
			value:        "hello",
			cursorPos:    5,
			expectValue:  "hell",
			expectCursor: 4,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			newValue, newCursor := applyGraphemeBackspace(tc.value, tc.cursorPos)
			if newValue != tc.expectValue {
				t.Errorf("expected value %q, got %q", tc.expectValue, newValue)
			}
			if newCursor != tc.expectCursor {
				t.Errorf("expected cursor %d, got %d", tc.expectCursor, newCursor)
			}
		})
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
