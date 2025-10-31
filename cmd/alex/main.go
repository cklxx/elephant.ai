package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]

	if handled, exitCode := handleStandaloneArgs(args); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}

	// Determine if we should disable sandbox for faster startup
	// Only disable for short-lived read-only commands
	// Task execution commands (default case) should respect sandbox config
	shouldDisableSandbox := shouldDisableSandboxForCommand(args)

	container, err := buildContainerWithOptions(shouldDisableSandbox)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Start the container (initializes MCP, Git tools, etc.)
	if err := container.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start container: %v\n", err)
		os.Exit(1)
	}

	// Ensure cleanup on exit
	defer func() {
		if err := container.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Cleanup error: %v\n", err)
		}
	}()

	// No arguments: enter interactive mode
	if len(args) == 0 {
		if err := RunNativeChatUI(container); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// With arguments: execute command
	cli := NewCLI(container)
	if err := cli.Run(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleStandaloneArgs(args []string) (handled bool, exitCode int) {
	if len(args) == 0 {
		return false, 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return true, 0
	case "version", "-v", "--version":
		fmt.Println(appVersion())
		return true, 0
	case "config":
		if err := runConfigCommand(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return true, 1
		}
		return true, 0
	}

	return false, 0
}

// shouldDisableSandboxForCommand determines if sandbox should be disabled for a command.
// Returns true only for short-lived read-only commands that don't need sandbox.
// Task execution commands (default case) return false to respect sandbox configuration.
func shouldDisableSandboxForCommand(args []string) bool {
	// No args means TUI mode - respect sandbox config
	if len(args) == 0 {
		return false
	}

	cmd := args[0]
	switch cmd {
	// Short-lived read-only commands - disable sandbox for faster startup
	case "sessions", "session":
		return true
	case "cost", "costs":
		return true
	case "mcp":
		return true

	// Task execution (default case) and other commands - respect sandbox config
	default:
		return false
	}
}
