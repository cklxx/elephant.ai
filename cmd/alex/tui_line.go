package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

const lineInputBufferSize = 1024 * 1024

type lineChatLoop struct {
	reader  *bufio.Reader
	out     io.Writer
	errOut  io.Writer
	runTask func(task string) error
	clear   func()
	header  func()
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

	ctx := cliBaseContext()
	session, err := container.AgentCoordinator.GetSession(ctx, "")
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	header := func() {
		printLineModeHeader(out)
	}
	clear := func() {
		clearLineModeScreen(out)
		header()
	}

	loop := &lineChatLoop{
		reader:  bufio.NewReaderSize(in, lineInputBufferSize),
		out:     out,
		errOut:  errOut,
		runTask: func(task string) error { return RunTaskWithStreamOutput(container, task, session.ID) },
		clear:   clear,
		header:  header,
	}

	loop.header()
	return loop.run()
}

func (l *lineChatLoop) run() error {
	if l == nil || l.reader == nil {
		return nil
	}

	for {
		l.printPrompt()
		line, ok, err := readLine(l.reader)
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
		case commandRun:
			if err := l.runCommand(cmd.task); err != nil {
				return err
			}
		default:
			continue
		}
	}
}

func (l *lineChatLoop) printPrompt() {
	if l == nil || l.out == nil {
		return
	}
	fmt.Fprint(l.out, styleBoldGreen.Render("❯ "))
}

func (l *lineChatLoop) runCommand(task string) error {
	if l.runTask == nil {
		return nil
	}
	if err := l.runTask(task); err != nil {
		if errors.Is(err, ErrForceExit) {
			return err
		}
		if l.errOut != nil {
			fmt.Fprintf(l.errOut, "%s %v\n", styleError.Render("Error:"), err)
		}
	}
	return nil
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
	fmt.Fprintf(out, "%s\n\n", styleGray.Render("commands: /quit, /exit, /clear"))
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
