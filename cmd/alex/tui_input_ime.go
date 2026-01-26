package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rivo/uniseg"
)

func shouldUseIMEInput(envLookup func(string) (string, bool)) bool {
	if envLookup == nil {
		envLookup = runtimeEnvLookup()
	}

	if value, ok := envLookup("ALEX_TUI_INPUT"); ok {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "raw", "fullscreen":
			return false
		case "ime", "cjk", "cooked", "line", "terminal":
			return true
		}
	}

	if value, ok := envLookup("ALEX_TUI_IME"); ok && envTruthy(value) {
		return true
	}

	if hasCJKLocale(envLookup) {
		return true
	}

	return false
}

// isBackspaceKey returns true if the key message represents a backspace action
func isBackspaceKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "ctrl+h", "backspace", "delete", "del":
		return true
	}

	switch msg.Type {
	case tea.KeyBackspace, tea.KeyDelete:
		return true
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 8, 127: // ctrl+h or DEL character
				return true
			}
		}
	}
	return false
}

// applyGraphemeBackspace performs grapheme-aware backspace on the given value and cursor position.
// Returns the new value and new cursor position.
func applyGraphemeBackspace(value string, cursorPos int) (string, int) {
	runes := []rune(value)
	if len(runes) == 0 || cursorPos == 0 {
		return value, cursorPos
	}

	// Clamp cursor position
	if cursorPos > len(runes) {
		cursorPos = len(runes)
	}

	// Find grapheme cluster boundaries
	graphemes := uniseg.NewGraphemes(value)
	var boundaries []int
	boundaries = append(boundaries, 0)

	runeIdx := 0
	for graphemes.Next() {
		runeCount := len([]rune(graphemes.Str()))
		runeIdx += runeCount
		boundaries = append(boundaries, runeIdx)
	}

	// Find the grapheme boundary before cursor position
	deleteFrom := 0
	for i := len(boundaries) - 1; i >= 0; i-- {
		if boundaries[i] < cursorPos {
			deleteFrom = boundaries[i]
			break
		}
	}

	// Build new value
	newRunes := make([]rune, 0, len(runes))
	newRunes = append(newRunes, runes[:deleteFrom]...)
	newRunes = append(newRunes, runes[cursorPos:]...)

	return string(newRunes), deleteFrom
}

