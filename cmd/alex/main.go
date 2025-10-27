package main

import (
	"fmt"
	"os"
)

func main() {

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

	args := os.Args[1:]

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
