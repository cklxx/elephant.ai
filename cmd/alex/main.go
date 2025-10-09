package main

import (
	"fmt"
	"os"
	"strings"

	"alex/cmd/alex/ui/eventhub"
	"alex/cmd/alex/ui/state"
	"alex/cmd/alex/ui/tviewui"
)

func main() {

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

	rawArgs := os.Args[1:]
	noTUI := shouldDisableTUI(rawArgs, container.Runtime.DisableTUI)
	filteredArgs := stripControlFlags(rawArgs)

	// Detect mode: interactive chat vs command execution
	switch {
	case len(rawArgs) == 0:
		if noTUI {
			if err := RunNativeChatUI(container); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			return
		}
		if err := RunInteractiveChatTUI(container); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case noTUI && len(filteredArgs) == 0:
		if err := RunNativeChatUI(container); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	default:
		cli := NewCLI(container)
		if err := cli.Run(filteredArgs); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

// RunInteractiveChatTUI starts the interactive chat interface
func RunInteractiveChatTUI(container *Container) error {
	followTranscript := container.Runtime.FollowTranscript
	followStream := container.Runtime.FollowStream

	cfg := tviewui.Config{
		Coordinator:      container.Coordinator,
		Store:            state.NewStore(),
		Hub:              eventhub.NewHub(),
		Registry:         container.MCPRegistry,
		Verbose:          container.Runtime.Verbose,
		CostTracker:      container.CostTracker,
		FollowTranscript: &followTranscript,
		FollowStream:     &followStream,
	}

	ui, err := tviewui.NewChatUI(cfg)
	if err != nil {
		return err
	}

	if err := ui.Run(); err != nil {
		// Fallback to native UI if the tview app cannot start
		return RunNativeChatUI(container)
	}
	return nil
}

func shouldDisableTUI(args []string, defaultDisable bool) bool {
	disable := defaultDisable

	for _, arg := range args {
		switch strings.ToLower(strings.TrimSpace(arg)) {
		case "--no-tui", "--native-tui", "--tui=false", "--tui=0":
			disable = true
		case "--tui", "--tui=true", "--tui=1":
			disable = false
		}
	}

	return disable
}

func stripControlFlags(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	var filtered []string
	for _, arg := range args {
		lower := strings.ToLower(arg)
		switch lower {
		case "--no-tui", "--native-tui", "--tui", "--tui=true", "--tui=false", "--tui=1", "--tui=0":
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
}
