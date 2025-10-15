package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"alex/cmd/alex/ui/eventhub"
	"alex/cmd/alex/ui/state"
	"alex/cmd/alex/ui/tviewui"
	"alex/internal/config"

	"github.com/gdamore/tcell/v2/terminfo"
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
	prepareTerminalForTUI(config.DefaultEnvLookup, os.Setenv, os.Stderr)

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

const defaultTERM = "xterm-256color"

var terminalAliasFallback = map[string]string{
	"alacritty":             defaultTERM,
	"alacritty-direct":      defaultTERM,
	"apple_terminal":        defaultTERM,
	"apple-terminal":        defaultTERM,
	"appleterm":             defaultTERM,
	"contour":               defaultTERM,
	"ghostty":               defaultTERM,
	"hyper":                 defaultTERM,
	"iterm.app":             defaultTERM,
	"kitty":                 defaultTERM,
	"tmux":                  defaultTERM,
	"tmux-256color":         defaultTERM,
	"tmux-24bit":            defaultTERM,
	"warp":                  defaultTERM,
	"warpterminal":          defaultTERM,
	"wezterm":               defaultTERM,
	"wezterm-direct":        defaultTERM,
	"wezterm-gui":           defaultTERM,
	"xterm-ghostty":         defaultTERM,
	"xterm-kitty":           defaultTERM,
	"zellij":                defaultTERM,
	"tabby":                 defaultTERM,
	"vscode":                defaultTERM,
	"vscode-terminal":       defaultTERM,
	"screen":                defaultTERM,
	"screen-256color":       defaultTERM,
	"screen.xterm-256color": defaultTERM,
}

func prepareTerminalForTUI(envLookup config.EnvLookup, setEnv func(string, string) error, stderr io.Writer) {
	prepareTerminalWithLookup(envLookup, setEnv, stderr, func(name string) error {
		if name == "" {
			return fmt.Errorf("empty terminal type")
		}
		_, err := terminfo.LookupTerminfo(name)
		return err
	})
}

func prepareTerminalWithLookup(envLookup config.EnvLookup, setEnv func(string, string) error, stderr io.Writer, lookup func(string) error) {
	var originalTERM, termProgram string
	if envLookup != nil {
		if value, ok := envLookup("TERM"); ok {
			originalTERM = value
		}
		if value, ok := envLookup("TERM_PROGRAM"); ok {
			termProgram = value
		}
	}

	normalized, changed, err := normalizeTerminal(originalTERM, termProgram, lookup)
	if err != nil || !changed {
		return
	}

	if err := setEnv("TERM", normalized); err != nil {
		_, _ = fmt.Fprintf(stderr, "Warning: unable to configure terminal fallback for interactive UI: %v\n", err)
		return
	}

	_, _ = fmt.Fprintf(stderr, "Detected unsupported TERM=%q; using %q for interactive chat UI.\n", originalTERM, normalized)
}

func normalizeTerminal(term, termProgram string, lookup func(string) error) (string, bool, error) {
	trimmed := strings.TrimSpace(term)
	if trimmed != "" {
		if err := lookup(trimmed); err == nil {
			return trimmed, false, nil
		}
	}

	candidates := candidateFallbackTerms(trimmed, termProgram)
	tried := map[string]struct{}{}
	for _, candidate := range candidates {
		cand := strings.TrimSpace(candidate)
		if cand == "" {
			continue
		}
		if _, seen := tried[cand]; seen {
			continue
		}
		tried[cand] = struct{}{}
		if err := lookup(cand); err == nil {
			return cand, true, nil
		}
	}

	if trimmed == "" {
		return "", false, fmt.Errorf("no compatible terminal detected")
	}
	return trimmed, false, fmt.Errorf("terminal %q unsupported", trimmed)
}

func candidateFallbackTerms(term, termProgram string) []string {
	var candidates []string

	if fallback := fallbackTermAlias(term); fallback != "" {
		candidates = append(candidates, fallback)
	}
	if fallback := fallbackTermAlias(termProgram); fallback != "" {
		candidates = append(candidates, fallback)
	}
	if fallback := fallbackForCommonNames(term); fallback != "" {
		candidates = append(candidates, fallback)
	}
	if fallback := fallbackForCommonNames(termProgram); fallback != "" {
		candidates = append(candidates, fallback)
	}
	candidates = append(candidates, defaultTERM)
	return candidates
}

func fallbackTermAlias(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	if key == "" {
		return ""
	}
	if alias, ok := terminalAliasFallback[key]; ok {
		return alias
	}
	return ""
}

func fallbackForCommonNames(value string) string {
	key := strings.ToLower(strings.TrimSpace(value))
	if key == "" {
		return ""
	}
	switch key {
	case "xterm", "xterm-color", "xterm-16color", "xterm-88color":
		return defaultTERM
	case "ansi", "vt100", "vt220":
		return defaultTERM
	case "dumb":
		return ""
	}
	if strings.HasSuffix(key, "-256color") {
		return key
	}
	if strings.Contains(key, "256color") {
		return defaultTERM
	}
	return ""
}
