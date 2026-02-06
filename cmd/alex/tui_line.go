package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	agentports "alex/internal/domain/agent/ports/agent"

	"golang.org/x/term"
)

const lineInputBufferSize = 1024 * 1024

type lineChatLoop struct {
	prompter linePrompter
	out      io.Writer
	errOut   io.Writer
	runTask  func(task string) (*agentports.TaskResult, error)
	selectUI func(question string, options []string) (string, bool, error)
	clear    func()
	header   func()

	abortCount int
	lastAbort  time.Time
}

func runLineChatUI(container *Container, in io.Reader, out io.Writer, errOut io.Writer) error {
	if container == nil {
		return fmt.Errorf("container is nil")
	}
	if in == nil {
		in = os.Stdin
	}
	if out == nil {
		out = os.Stdout
	}
	if errOut == nil {
		errOut = os.Stderr
	}

	interactive := isInteractiveTTY(in, out)
	ctx := cliBaseContext()
	session, err := container.AgentCoordinator.GetSession(ctx, "")
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	header := func() {
		if interactive {
			printLineModeHeader(out)
		}
	}
	clear := func() {
		if interactive {
			clearLineModeScreen(out)
			header()
		}
	}

	prompt := styleBoldGreen.Render("❯ ")
	prompter := buildLinePrompter(in, out, errOut, prompt, interactive, historyFilePath(container))
	defer func() {
		_ = prompter.Close()
	}()

	loop := &lineChatLoop{
		prompter: prompter,
		out:      out,
		errOut:   errOut,
		runTask: func(task string) (*agentports.TaskResult, error) {
			return RunTaskWithStreamOutputResult(container, task, session.ID)
		},
		selectUI: newAwaitChoiceSelector(in, out, interactive).Select,
		clear:    clear,
		header:   header,
	}

	loop.header()
	return loop.run()
}

func (l *lineChatLoop) run() error {
	if l == nil || l.prompter == nil {
		return nil
	}

	for {
		line, ok, err := l.readPrompt()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		cmd := parseUserCommand(line)
		switch cmd.kind {
		case commandEmpty:
			continue
		case commandQuit:
			return nil
		case commandClear:
			if l.clear != nil {
				l.clear()
			}
			continue
		case commandHelp:
			if l.out != nil {
				printLineModeHelp(l.out)
			}
			continue
		case commandRun:
			if l.prompter != nil {
				l.prompter.AppendHistory(cmd.task)
			}
			if err := l.runCommand(cmd.task); err != nil {
				return err
			}
		default:
			continue
		}
	}
}

func (l *lineChatLoop) readPrompt() (string, bool, error) {
	if l == nil || l.prompter == nil {
		return "", false, nil
	}
	line, ok, err := l.prompter.Prompt()
	if err == nil {
		l.abortCount = 0
		return line, ok, nil
	}
	if errors.Is(err, errPromptAborted) {
		if l.shouldExitOnAbort() {
			return "", false, nil
		}
		if l.out != nil {
			fmt.Fprintln(l.out, styleGray.Render("Press Ctrl+C again to exit."))
		}
		return "", true, nil
	}
	return "", false, err
}

func (l *lineChatLoop) shouldExitOnAbort() bool {
	now := time.Now()
	if l.lastAbort.IsZero() || now.Sub(l.lastAbort) > time.Second {
		l.lastAbort = now
		l.abortCount = 1
		return false
	}
	l.abortCount++
	return l.abortCount >= 2
}

func (l *lineChatLoop) runCommand(task string) error {
	if l.runTask == nil {
		return nil
	}

	pending := task
	for {
		result, err := l.runTask(pending)
		if err != nil {
			if errors.Is(err, ErrForceExit) {
				return err
			}
			if l.errOut != nil {
				fmt.Fprintf(l.errOut, "%s %v\n", styleError.Render("Error:"), err)
			}
			return nil
		}
		prompt, ok := extractAwaitPrompt(result)
		if !ok || len(prompt.Options) == 0 || l.selectUI == nil {
			return nil
		}

		selection, selected, err := l.selectUI(prompt.Question, prompt.Options)
		if err != nil {
			if errors.Is(err, errPromptAborted) {
				if l.errOut != nil {
					fmt.Fprintln(l.errOut, styleGray.Render("Selection cancelled."))
				}
				return nil
			}
			return err
		}
		if !selected {
			return nil
		}
		pending = selection
		if l.prompter != nil {
			l.prompter.AppendHistory(selection)
		}
	}
}

func extractAwaitPrompt(result *agentports.TaskResult) (agentports.AwaitUserInputPrompt, bool) {
	if result == nil || !strings.EqualFold(strings.TrimSpace(result.StopReason), "await_user_input") {
		return agentports.AwaitUserInputPrompt{}, false
	}
	return agentports.ExtractAwaitUserInputPrompt(result.Messages)
}

func readLine(reader *bufio.Reader) (string, bool, error) {
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false, err
	}
	if errors.Is(err, io.EOF) && line == "" {
		return "", false, nil
	}
	return strings.TrimRight(line, "\r\n"), true, nil
}

func printLineModeHeader(out io.Writer) {
	if out == nil {
		return
	}

	fmt.Fprintf(out, "%s %s\n", styleBold.Render(styleGreen.Render(tuiAgentName)), styleGray.Render("— interactive"))
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		fmt.Fprintf(out, "%s %s\n", styleGray.Render("cwd:"), cwd)
	}
	if branch := currentGitBranch(); branch != "" {
		fmt.Fprintf(out, "%s %s\n", styleGray.Render("git:"), styleGreen.Render(branch))
	}
	fmt.Fprintf(out, "%s\n\n", styleGray.Render("commands: /help, /quit, /exit, /clear"))
}

func printLineModeHelp(out io.Writer) {
	if out == nil {
		return
	}
	fmt.Fprintln(out, styleGray.Render("Commands:"))
	for _, cmd := range lineModeCommands() {
		fmt.Fprintf(out, "  %s\n", styleGreen.Render(cmd))
	}
	fmt.Fprintln(out, styleGray.Render("Tips: Ctrl+D to exit, Ctrl+C twice to quit, ↑/↓ for history."))
	fmt.Fprintln(out)
}

func clearLineModeScreen(out io.Writer) {
	if out == nil {
		return
	}
	if file, ok := out.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		fmt.Fprint(out, "\033[2J\033[H")
		return
	}
	fmt.Fprint(out, "\n")
}

func isInteractiveTTY(in io.Reader, out io.Writer) bool {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK {
		return false
	}
	return term.IsTerminal(int(inFile.Fd())) && term.IsTerminal(int(outFile.Fd()))
}

func buildLinePrompter(in io.Reader, out io.Writer, errOut io.Writer, prompt string, interactive bool, historyPath string) linePrompter {
	if !interactive {
		if reader, ok := in.(*bufio.Reader); ok {
			return newBufferedPrompter(reader, prompt, out, false)
		}
		return newBufferedPrompter(bufio.NewReaderSize(in, lineInputBufferSize), prompt, out, false)
	}
	linerPrompt, err := newLinerPrompter(prompt, historyPath, errOut)
	if err != nil {
		if errOut != nil {
			fmt.Fprintf(errOut, "Warning: readline disabled: %v\n", err)
		}
		return newBufferedPrompter(bufio.NewReaderSize(in, lineInputBufferSize), prompt, out, true)
	}
	return linerPrompt
}

func lineModeCommands() []string {
	return []string{"/help", "/quit", "/exit", "/clear"}
}
