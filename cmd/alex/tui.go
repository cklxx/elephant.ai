package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"alex/internal/output"

	"golang.org/x/term"
)

// RunNativeChatUI starts the interactive chat UI. It prefers the full-screen
// Bubble Tea TUI and falls back to a simple line-mode loop when TTY features
// are unavailable or disabled.
func RunNativeChatUI(container *Container) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}

	output.ConfigureCLIColorProfile(os.Stdout)

	if !container.Runtime.DisableTUI && shouldUseFullscreenTUI() && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		if err := RunBubbleChatUI(container); err == nil {
			return nil
		}
	}

	return runLineChatUI(container)
}

func shouldUseFullscreenTUI() bool {
	envLookup := runtimeEnvLookup()
	if shouldForceLineInput(envLookup) {
		return false
	}
	mode, _ := envLookup("ALEX_TUI_MODE")
	mode = strings.TrimSpace(mode)
	if strings.EqualFold(mode, "fullscreen") || strings.EqualFold(mode, "full") {
		return true
	}
	if strings.EqualFold(mode, "terminal") || strings.EqualFold(mode, "inline") {
		return false
	}
	fullscreen, _ := envLookup("ALEX_TUI_FULLSCREEN")
	switch strings.ToLower(strings.TrimSpace(fullscreen)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func shouldForceLineInput(envLookup func(string) (string, bool)) bool {
	if envLookup == nil {
		envLookup = runtimeEnvLookup()
	}

	if value, ok := envLookup("ALEX_TUI_INPUT"); ok {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "ime":
			return false
		case "cooked", "line", "terminal":
			return true
		case "raw", "fullscreen":
			return false
		}
	}

	if value, ok := envLookup("ALEX_TUI_IME"); ok && envTruthy(value) {
		return false
	}

	if hasCJKLocale(envLookup) {
		return true
	}

	return false
}

func envTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func hasCJKLocale(envLookup func(string) (string, bool)) bool {
	if envLookup == nil {
		envLookup = runtimeEnvLookup()
	}

	if value, ok := envLookup("LC_ALL"); ok && isCJKLocale(value) {
		return true
	}
	if value, ok := envLookup("LC_CTYPE"); ok && isCJKLocale(value) {
		return true
	}
	if value, ok := envLookup("LANG"); ok && isCJKLocale(value) {
		return true
	}
	return false
}

func isCJKLocale(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return false
	}
	if idx := strings.IndexAny(trimmed, ".@"); idx >= 0 {
		trimmed = trimmed[:idx]
	}
	switch {
	case strings.HasPrefix(trimmed, "zh"):
		return true
	case strings.HasPrefix(trimmed, "ja"):
		return true
	case strings.HasPrefix(trimmed, "ko"):
		return true
	default:
		return false
	}
}

func runLineChatUI(container *Container) error {
	session, err := container.SessionStore.Create(context.Background())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	fmt.Printf("%s %s\n", styleBold.Render(styleGreen.Render(tuiAgentName)), styleGray.Render("— interactive"))
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		fmt.Printf("%s %s\n", styleGray.Render("cwd:"), cwd)
	}
	if branch := currentGitBranch(); branch != "" {
		fmt.Printf("%s %s\n", styleGray.Render("git:"), styleGreen.Render(branch))
	}
	fmt.Printf("%s\n\n", styleGray.Render("commands: /quit, /exit, /clear"))

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 1024*1024)

	for {
		fmt.Print(styleBoldGreen.Render("❯ "))
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return nil
		}

		task := strings.TrimSpace(scanner.Text())
		if task == "" {
			continue
		}

		switch task {
		case "/quit", "/exit":
			return nil
		case "/clear":
			fmt.Print("\033[2J\033[H")
			continue
		}

		if err := RunTaskWithStreamOutput(container, task, session.ID); err != nil {
			if err == ErrForceExit {
				return err
			}
			fmt.Fprintf(os.Stderr, "%s %v\n", styleError.Render("Error:"), err)
		}
	}
}
