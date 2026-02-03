package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	runtimeconfig "alex/internal/config"
)

func main() {
	args := os.Args[1:]

	if err := runtimeconfig.LoadDotEnv(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to load .env: %v\n", err)
	}

	var shutdownOnce sync.Once
	shutdown := func(container *Container) {
		shutdownOnce.Do(func() {
			cancelCLIBaseContext()
			if container != nil {
				drainCtx, drainCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer drainCancel()
				if err := container.Drain(drainCtx); err != nil {
					fmt.Fprintf(os.Stderr, "Shutdown error: %v\n", err)
				}
			}
		})
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

	// Handle SIGTERM/SIGINT for graceful shutdown in CLI mode.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(quit)
	go func() {
		<-quit
		shutdown(container)
		os.Exit(130)
	}()

	// Start the container (initializes MCP, Git tools, etc.)
	if err := container.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start container: %v\n", err)
		os.Exit(1)
	}

	cleanup := func() { shutdown(container) }
	defer cleanup()

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
		var exitErr *ExitCodeError
		if errors.As(err, &exitErr) && exitErr.Code != 0 {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			cleanup()
			os.Exit(exitErr.Code)
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
	case "lark":
		if err := runLarkCommand(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return true, exitCodeFromError(err)
		}
		return true, 0
	}

	return false, 0
}

func exitCodeFromError(err error) int {
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) && exitErr.Code != 0 {
		return exitErr.Code
	}
	return 1
}
