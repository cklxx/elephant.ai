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
