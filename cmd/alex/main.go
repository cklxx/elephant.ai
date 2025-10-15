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
	_ "github.com/gdamore/tcell/v2/terminfo/extended"
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
	"tabby":                 defaultTERM,
	"tmux":                  defaultTERM,
	"tmux-256color":         defaultTERM,
	"tmux-24bit":            defaultTERM,
	"vscode":                defaultTERM,
	"vscode-terminal":       defaultTERM,
	"warp":                  defaultTERM,
	"warpterminal":          defaultTERM,
	"wezterm":               "wezterm",
	"wezterm-direct":        "wezterm",
	"wezterm-gui":           "wezterm",
	"xterm-ghostty":         defaultTERM,
	"xterm-kitty":           defaultTERM,
	"zellij":                defaultTERM,
	"screen":                defaultTERM,
	"screen-256color":       defaultTERM,
	"screen.xterm-256color": "xterm-256color",
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
		if _, ferr := fmt.Fprintf(stderr, "Warning: unable to configure terminal fallback for interactive UI: %v\n", err); ferr != nil {
			_ = ferr // Best-effort warning; ignore secondary failure.
		}
		return
	}

	if _, ferr := fmt.Fprintf(stderr, "Detected unsupported TERM=%q; using %q for interactive chat UI.\n", originalTERM, normalized); ferr != nil {
		_ = ferr // Best-effort notification; ignore secondary failure.
	}
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
	seen := map[string]struct{}{}
	var candidates []string

	appendCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		candidates = append(candidates, value)
	}

	addCandidates := func(value string) {
		for _, candidate := range fallbackCandidatesForValue(value) {
			appendCandidate(candidate)
		}
	}

	addCandidates(term)
	addCandidates(termProgram)
	appendCandidate(defaultTERM)

	return candidates
}

func fallbackCandidatesForValue(value string) []string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	lower := strings.ToLower(trimmed)
	var candidates []string

	if alias := fallbackTermAlias(lower); alias != "" {
		candidates = append(candidates, alias)
	}

	candidates = append(candidates, fallbackVariants(lower)...)

	if fallback := fallbackForCommonNames(lower); fallback != "" {
		candidates = append(candidates, fallback)
	}

	return candidates
}

func fallbackVariants(value string) []string {
	key := strings.TrimSpace(value)
	if key == "" {
		return nil
	}

	var variants []string
	base := key

	stripSuffix := func(suffix string) {
		if strings.HasSuffix(base, suffix) {
			trimmed := strings.TrimSuffix(base, suffix)
			if trimmed != "" {
				variants = append(variants, trimmed)
				base = trimmed
			}
		}
	}

	// Remove truecolor extensions before any additional derivations.
	for _, suffix := range []string{"-direct", "-truecolor", "-24bit"} {
		stripSuffix(suffix)
	}

	// Handle -256color specifically so we keep the colour-aware variant first.
	if strings.HasSuffix(key, "-256color") {
		trimmed := strings.TrimSuffix(key, "-256color")
		if trimmed != "" {
			variants = append(variants, trimmed)
		}
	}

	if idx := strings.LastIndex(base, "."); idx >= 0 && idx < len(base)-1 {
		variants = append(variants, base[idx+1:])
	}

	if idx := strings.LastIndex(base, "_"); idx >= 0 && idx < len(base)-1 {
		variants = append(variants, strings.ReplaceAll(base, "_", "-"))
	}

	return variants
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
