package main

import (
	"errors"
	"fmt"
	"os"

	runtimeconfig "alex/internal/config"
)

func main() {
	args := os.Args[1:]

	if err := runtimeconfig.LoadDotEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load .env: %v\n", err)
	}

	if handled, exitCode := handleStandaloneArgs(args); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}

	container, err := buildContainer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Start the container (initializes MCP, Git tools, etc.)
	if err := container.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start container: %v\n", err)
		os.Exit(1)
	}

	cleanup := func() {
		if err := container.Shutdown(); err != nil {
			fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
		}
	}
	defer func() {
		cleanup()
	}()

	// No arguments: enter interactive mode
	if len(args) == 0 {
		if err := RunNativeChatUI(container); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			cleanup()
			os.Exit(1)
		}
		return
	}

	// With arguments: execute command
	cli := NewCLI(container)
	if err := cli.Run(args); err != nil {
		if errors.Is(err, ErrForceExit) {
			cleanup()
			os.Exit(130)
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cleanup()
		os.Exit(1)
	}
}

func handleStandaloneArgs(args []string) (handled bool, exitCode int) {
	if len(args) == 0 {
		return false, 0
	}

	for _, arg := range args {
		if arg == "help" || arg == "-h" || arg == "--help" {
			printUsage()
			return true, 0
		}
	}

	switch args[0] {
	case "version", "-v", "--version":
		fmt.Println(appVersion())
		return true, 0
	case "config":
		if err := runConfigCommand(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return true, 1
		}
		return true, 0
	}

	return false, 0
}
