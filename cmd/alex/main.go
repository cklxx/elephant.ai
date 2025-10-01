package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	rand.Seed(time.Now().UnixNano())

	container, err := buildContainer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Ensure cleanup on exit
	defer func() {
		if err := container.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Cleanup error: %v\n", err)
		}
	}()

	// Detect mode: interactive chat vs command execution
	if len(os.Args) == 1 {
		// No arguments → Interactive Chat TUI
		if err := RunInteractiveChatTUI(container); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Has arguments → Execute as command with stream output
		cli := NewCLI(container)
		if err := cli.Run(os.Args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// RunInteractiveChatTUI starts the interactive chat interface
func RunInteractiveChatTUI(container *Container) error {
	// Create a new session
	ctx := context.Background()
	_, err := container.Coordinator.GetSession(ctx, "") // Empty ID creates new session
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	// Use streaming TUI model instead
	model := initialStreamingModel()

	// Create Bubble Tea program
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	// Run (blocks until quit)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	return nil
}
