package main

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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

func applyIMEKey(buffer []rune, msg tea.KeyMsg) ([]rune, bool) {
	switch msg.Type {
	case tea.KeyRunes:
		if len(msg.Runes) == 0 {
			return buffer, true
		}
		buffer = append(buffer, msg.Runes...)
		return buffer, true
	case tea.KeySpace:
		buffer = append(buffer, ' ')
		return buffer, true
	case tea.KeyTab:
		buffer = append(buffer, '\t')
		return buffer, true
	case tea.KeyBackspace, tea.KeyDelete:
		if len(buffer) == 0 {
			return buffer, true
		}
		buffer = buffer[:len(buffer)-1]
		return buffer, true
	default:
		return buffer, false
	}
}
