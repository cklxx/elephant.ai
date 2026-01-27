package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"alex/internal/output"

	"golang.org/x/term"
)

// RunNativeChatUI starts the interactive chat UI. It prefers the full-screen
// gocui TUI and falls back to a simple line-mode loop when TTY features
// are unavailable or disabled.
func RunNativeChatUI(container *Container) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}

	if !container.Runtime.DisableTUI && shouldUseFullscreenTUI() && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		if err := RunGocui(container); err == nil {
			return nil
		}
	}

	return runLineChatUI(container)
}

func shouldUseFullscreenTUI() bool {
	envLookup := runtimeEnvLookup()

	// Check explicit mode setting first
	mode, _ := envLookup("ALEX_TUI_MODE")
	mode = strings.TrimSpace(mode)
	if strings.EqualFold(mode, "fullscreen") || strings.EqualFold(mode, "full") || strings.EqualFold(mode, "gocui") {
		return true
	}
	if strings.EqualFold(mode, "terminal") || strings.EqualFold(mode, "inline") || strings.EqualFold(mode, "line") {
		return false
	}

	// Check explicit fullscreen setting
	fullscreen, _ := envLookup("ALEX_TUI_FULLSCREEN")
	switch strings.ToLower(strings.TrimSpace(fullscreen)) {
	case "0", "false", "no", "off":
		return false
	case "1", "true", "yes", "on":
		return true
	}

	// Default to fullscreen TUI.
	return true
}

func runLineChatUI(container *Container) error {
	output.ConfigureCLIColorProfile(os.Stdout)

	session, err := container.SessionStore.Create(cliBaseContext())
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

		cmd := parseUserCommand(scanner.Text())
		switch cmd.kind {
		case commandEmpty:
			continue
		case commandQuit:
			return nil
		case commandClear:
			fmt.Print("\033[2J\033[H")
			continue
		case commandRun:
			if err := RunTaskWithStreamOutput(container, cmd.task, session.ID); err != nil {
				if err == ErrForceExit {
					return err
				}
				fmt.Fprintf(os.Stderr, "%s %v\n", styleError.Render("Error:"), err)
			}
		default:
			continue
		}
	}
}
