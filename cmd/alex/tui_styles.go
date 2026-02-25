package main

import "github.com/charmbracelet/lipgloss"

// Shared TUI styles for line-mode CLI.
var (
	styleGray      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleGreen     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleBoldGreen = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
)
