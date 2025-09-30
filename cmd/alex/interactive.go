package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	markdown "github.com/MichaelMure/go-term-markdown"
	"github.com/chzyer/readline"
)

// RunInteractive runs a simple REPL with readline support (arrow keys, history)
func RunInteractive(container *Container) error {
	fmt.Println("ALEX - AI Code Agent")
	fmt.Println("Type your task and press Enter. Type 'exit' or 'quit' to quit.")
	fmt.Println("Use ↑/↓ arrow keys to navigate command history.")
	fmt.Println()

	// Create a persistent session for this interactive session
	ctx := context.Background()
	session, err := container.SessionStore.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Printf("Session ID: %s\n", session.ID)
	fmt.Println()

	// Setup readline with history
	homeDir, _ := os.UserHomeDir()
	historyFile := filepath.Join(homeDir, ".alex-history")

	rl, err := readline.NewEx(&readline.Config{
		Prompt:            "> ",
		HistoryFile:       historyFile,
		InterruptPrompt:   "^C",
		EOFPrompt:         "exit",
		HistorySearchFold: true,
		AutoComplete:      nil,
		UniqueEditLine:    true,

		// Important: Use stdin/stdout/stderr explicitly
		Stdin:  readline.NewCancelableStdin(os.Stdin),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize readline: %w", err)
	}
	defer rl.Close()

	for {
		// Read input with arrow key support
		input, err := rl.Readline()
		if err == readline.ErrInterrupt {
			// Ctrl+C pressed
			if len(input) == 0 {
				fmt.Println("\nGoodbye!")
				break
			} else {
				// Clear current input
				continue
			}
		} else if err == io.EOF {
			// Ctrl+D pressed
			fmt.Println("\nGoodbye!")
			break
		}

		input = strings.TrimSpace(input)

		// Check for exit commands
		if input == "exit" || input == "quit" || input == "q" {
			fmt.Println("Goodbye!")
			break
		}

		// Skip empty input
		if input == "" {
			continue
		}

		// Execute task with persistent session ID
		result, err := container.Coordinator.ExecuteTask(ctx, input, session.ID)
		if err != nil {
			fmt.Printf("\nError: %v\n\n", err)
			continue
		}

		// Display answer with markdown rendering
		if result.Answer != "" {
			rendered := renderMarkdown(result.Answer)
			fmt.Printf("\n%s\n", rendered)
		}

		// Display stats
		fmt.Printf("\n✓ Completed in %d iterations, %d tokens\n\n", result.Iterations, result.TokensUsed)
	}

	return nil
}

// renderMarkdown renders markdown content to terminal
func renderMarkdown(content string) string {
	// Get terminal width (default 80)
	width := 100

	// Render markdown with syntax highlighting
	result := markdown.Render(content, width, 6) // 6 = left padding
	return string(result)
}
