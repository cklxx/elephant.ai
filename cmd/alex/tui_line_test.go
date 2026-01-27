package main

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

type fakePrompter struct {
	lines       []string
	index       int
	history     []string
	promptError error
}

func (p *fakePrompter) Prompt() (string, bool, error) {
	if p.promptError != nil {
		err := p.promptError
		p.promptError = nil
		return "", false, err
	}
	if p.index >= len(p.lines) {
		return "", false, nil
	}
	line := p.lines[p.index]
	p.index++
	return line, true, nil
}

func (p *fakePrompter) AppendHistory(entry string) {
	p.history = append(p.history, entry)
}

func (p *fakePrompter) Close() error { return nil }

func TestLineChatLoopQuit(t *testing.T) {
	t.Parallel()

	prompter := &fakePrompter{lines: []string{"/quit"}}
	loop := &lineChatLoop{
		prompter: prompter,
		out:      io.Discard,
		runTask: func(task string) error {
			t.Fatalf("runTask should not be called, got %q", task)
			return nil
		},
	}

	if err := loop.run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLineChatLoopRunsCommandsAndClear(t *testing.T) {
	t.Parallel()

	var tasks []string
	cleared := 0

	prompter := &fakePrompter{lines: []string{"", "  hello  ", "/clear", "/exit"}}
	loop := &lineChatLoop{
		prompter: prompter,
		out:      io.Discard,
		errOut:   io.Discard,
		runTask: func(task string) error {
			tasks = append(tasks, task)
			return nil
		},
		clear: func() {
			cleared++
		},
	}

	if err := loop.run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tasks) != 1 || tasks[0] != "hello" {
		t.Fatalf("expected task [hello], got %#v", tasks)
	}
	if cleared != 1 {
		t.Fatalf("expected clear to be called once, got %d", cleared)
	}
	if len(prompter.history) != 1 || prompter.history[0] != "hello" {
		t.Fatalf("expected history [hello], got %#v", prompter.history)
	}
}

func TestLineChatLoopPropagatesForceExit(t *testing.T) {
	t.Parallel()

	prompter := &fakePrompter{lines: []string{"run"}}
	loop := &lineChatLoop{
		prompter: prompter,
		out:      io.Discard,
		errOut:   io.Discard,
		runTask: func(task string) error {
			if task != "run" {
				t.Fatalf("unexpected task: %q", task)
			}
			return ErrForceExit
		},
	}

	if err := loop.run(); err != ErrForceExit {
		t.Fatalf("expected ErrForceExit, got %v", err)
	}
}

func TestLineChatLoopPromptAbort(t *testing.T) {
	t.Parallel()

	prompter := &fakePrompter{
		lines:       []string{"/quit"},
		promptError: errPromptAborted,
	}
	loop := &lineChatLoop{
		prompter: prompter,
		out:      io.Discard,
	}

	if err := loop.run(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestReadLineHandlesEOF(t *testing.T) {
	t.Parallel()

	reader := bufio.NewReader(strings.NewReader("last line"))
	line, ok, err := readLine(reader)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok || line != "last line" {
		t.Fatalf("expected last line, got ok=%v line=%q", ok, line)
	}

	line, ok, err = readLine(reader)
	if err != nil {
		t.Fatalf("unexpected error after eof: %v", err)
	}
	if ok || line != "" {
		t.Fatalf("expected eof, got ok=%v line=%q", ok, line)
	}
}
