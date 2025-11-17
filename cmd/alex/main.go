package main

import (
	"errors"
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	cliOptions, args, err := parseGlobalCLIOptions(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(2)
	}

	if handled, exitCode := handleStandaloneArgs(args); handled {
		if exitCode != 0 {
			os.Exit(exitCode)
		}
		return
	}

	// Always disable sandbox execution in CLI mode to ensure tools run locally.
	shouldDisableSandbox := true

	container, err := buildContainerWithOptions(shouldDisableSandbox, cliOptions.loaderOptions()...)
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
		if err := container.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Cleanup error: %v\n", err)
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
