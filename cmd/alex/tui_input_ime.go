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

func applyIMEKey(buffer []rune, cursorPos int, msg tea.KeyMsg) (newBuffer []rune, newCursorPos int, handled bool) {
	switch msg.String() {
	case "ctrl+h", "backspace", "delete", "del":
		newBuffer, newCursorPos = deleteGraphemeAtCursor(buffer, cursorPos)
		return newBuffer, newCursorPos, true
	}

	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) == 1 {
			switch msg.Runes[0] {
			case 8, 127:
				newBuffer, newCursorPos = deleteGraphemeAtCursor(buffer, cursorPos)
				return newBuffer, newCursorPos, true
			}
		}
		if len(msg.Runes) == 0 {
			return buffer, cursorPos, true
		}
		// Insert at cursor position
		newBuffer = make([]rune, 0, len(buffer)+len(msg.Runes))
		newBuffer = append(newBuffer, buffer[:cursorPos]...)
		newBuffer = append(newBuffer, msg.Runes...)
		newBuffer = append(newBuffer, buffer[cursorPos:]...)
		return newBuffer, cursorPos + len(msg.Runes), true
	case tea.KeySpace:
		newBuffer = make([]rune, 0, len(buffer)+1)
		newBuffer = append(newBuffer, buffer[:cursorPos]...)
		newBuffer = append(newBuffer, ' ')
		newBuffer = append(newBuffer, buffer[cursorPos:]...)
		return newBuffer, cursorPos + 1, true
	case tea.KeyTab:
		newBuffer = make([]rune, 0, len(buffer)+1)
		newBuffer = append(newBuffer, buffer[:cursorPos]...)
		newBuffer = append(newBuffer, '\t')
		newBuffer = append(newBuffer, buffer[cursorPos:]...)
		return newBuffer, cursorPos + 1, true
	case tea.KeyBackspace, tea.KeyDelete:
		newBuffer, newCursorPos = deleteGraphemeAtCursor(buffer, cursorPos)
		return newBuffer, newCursorPos, true
	default:
		return buffer, cursorPos, false
	}
}

// deleteGraphemeAtCursor deletes the grapheme cluster before the cursor position
// cursorPos is in rune index, not byte index
func deleteGraphemeAtCursor(buffer []rune, cursorPos int) ([]rune, int) {
	if len(buffer) == 0 || cursorPos == 0 {
		return buffer, cursorPos
	}

	// Convert to string for grapheme analysis
	content := string(buffer)

	// Build a map from byte position to rune position
	byteToRune := make(map[int]int)
	runePos := 0
	for bytePos := range content {
		byteToRune[bytePos] = runePos
		runePos++
	}
	byteToRune[len(content)] = len(buffer)

	// Find all grapheme boundaries (in byte positions)
	graphemes := uniseg.NewGraphemes(content)
	type boundary struct {
		bytePos int
		runePos int
	}
	var boundaries []boundary
	boundaries = append(boundaries, boundary{0, 0})

	for graphemes.Next() {
		_, endByte := graphemes.Positions()
		if endByte <= len(content) {
			boundaries = append(boundaries, boundary{endByte, byteToRune[endByte]})
		}
	}

	// Find the grapheme boundary before cursor position (in rune index)
	deleteFromRune := 0
	for i := len(boundaries) - 1; i >= 0; i-- {
		if boundaries[i].runePos < cursorPos {
			deleteFromRune = boundaries[i].runePos
			break
		}
	}

	// Delete the grapheme cluster (using rune indices)
	newBuffer := make([]rune, 0, len(buffer))
	newBuffer = append(newBuffer, buffer[:deleteFromRune]...)
	newBuffer = append(newBuffer, buffer[cursorPos:]...)

	return newBuffer, deleteFromRune
}

func deleteLastGrapheme(buffer []rune) []rune {
	if len(buffer) == 0 {
		return buffer
	}
	content := string(buffer)
	graphemes := uniseg.NewGraphemes(content)
	lastStart := -1
	for graphemes.Next() {
		start, _ := graphemes.Positions()
		lastStart = start
	}
	if lastStart <= 0 {
		return nil
	}
	return []rune(content[:lastStart])
}
