package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"
)

// RunNativeChatUI starts the interactive chat UI. It prefers the full-screen
// Bubble Tea TUI and falls back to a simple line-mode loop when TTY features
// are unavailable or disabled.
func RunNativeChatUI(container *Container) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}

	if !container.Runtime.DisableTUI && term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd())) {
		if err := RunBubbleChatUI(container); err == nil {
			return nil
		}
	}

	return runLineChatUI(container)
}

func runLineChatUI(container *Container) error {
	session, err := container.SessionStore.Create(context.Background())
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	fmt.Printf("%s %s\n", styleBold.Render(styleGreen.Render(tuiAgentName)), styleGray.Render("— interactive (line mode)"))
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
