package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// RunSimpleREPL runs a simple REPL without readline (no history, but guaranteed to work)
func RunSimpleREPL(container *Container) error {
	fmt.Println("ALEX - AI Code Agent (Simple Mode)")
	fmt.Println("Type your task and press Enter. Type 'exit' or 'quit' to quit.")
	fmt.Println()

	// Create a persistent session
	ctx := context.Background()
	session, err := container.SessionStore.Create(ctx)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Printf("Session ID: %s\n", session.ID)
	fmt.Println()

	// Simple scanner-based input
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for {
		// Print prompt
		fmt.Print("> ")

		// Read input
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("failed to read input: %w", err)
			}
			break
		}

		input := strings.TrimSpace(scanner.Text())

		// Check for exit
		if input == "exit" || input == "quit" || input == "q" {
			fmt.Println("Goodbye!")
			break
		}

		// Skip empty
		if input == "" {
			continue
		}

		// Execute task
		result, err := container.Coordinator.ExecuteTask(ctx, input, session.ID)
		if err != nil {
			fmt.Printf("\nError: %v\n\n", err)
			continue
		}

		// Display answer
		if result.Answer != "" {
			rendered := renderMarkdown(result.Answer)
			fmt.Printf("\n%s\n", rendered)
		}

		// Display stats
		fmt.Printf("\n✓ Completed in %d iterations, %d tokens\n\n", result.Iterations, result.TokensUsed)
	}

	return nil
}
