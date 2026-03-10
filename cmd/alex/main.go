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

	runtimeconfig "alex/internal/shared/config"
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
				if err := container.Container.Drain(drainCtx); err != nil {
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

	// Start the container lifecycle.
	if err := container.Container.Start(); err != nil {
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
		exitCode, printError := cliExitBehaviorFromError(err)
		if printError {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		cleanup()
		os.Exit(exitCode)
	}
}

func handleStandaloneArgs(args []string) (handled bool, exitCode int) {
	if len(args) == 0 {
		return false, 0
	}

	if isTopLevelHelp(args) {
		printUsage()
		return true, 0
	}

	switch args[0] {
	case "version", "-v", "--version":
		fmt.Println(appVersion())
		return true, 0
	case "config":
		return runStandaloneCommand(args[1:], func(subArgs []string) error {
			return executeConfigCommand(subArgs, os.Stdout)
		}, func(error) int { return 1 })
	case "dev":
		return runStandaloneCommand(args[1:], runDevCommand, exitCodeFromError)
	case "lark":
		return runStandaloneCommand(args[1:], runLarkCommand, exitCodeFromError)
	case "team":
		return runStandaloneCommand(args[1:], runTeamCommand, exitCodeFromError)
	case "runtime":
		return runStandaloneCommand(args[1:], runRuntimeCommand, exitCodeFromError)
	case "health":
		return runStandaloneCommand(args[1:], runHealthCommand, exitCodeFromError)
	}

	return false, 0
}

func isTopLevelHelp(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func runStandaloneCommand(args []string, runner func([]string) error, resolveExitCode func(error) int) (handled bool, exitCode int) {
	if err := runner(args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return true, resolveExitCode(err)
	}
	return true, 0
}

func exitCodeFromError(err error) int {
	var exitErr *ExitCodeError
	if errors.As(err, &exitErr) && exitErr.Code != 0 {
		return exitErr.Code
	}
	return 1
}

func cliExitBehaviorFromError(err error) (exitCode int, printError bool) {
	if errors.Is(err, ErrForceExit) {
		return 130, false
	}
	return exitCodeFromError(err), true
}
