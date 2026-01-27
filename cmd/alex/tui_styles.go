package main

import "github.com/charmbracelet/lipgloss"

// Shared TUI styles (gocui + line-mode fallback).
var (
	styleGray      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleGreen     = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	styleBold      = lipgloss.NewStyle().Bold(true)
	styleBoldCyan  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	styleError     = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleSystem    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)
