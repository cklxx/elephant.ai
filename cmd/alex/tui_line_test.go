package main

import (
	"bufio"
	"io"
	"strings"
	"testing"
)

func TestLineChatLoopQuit(t *testing.T) {
	t.Parallel()

	loop := &lineChatLoop{
		reader: bufio.NewReader(strings.NewReader("/quit\n")),
		out:    io.Discard,
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

	loop := &lineChatLoop{
		reader: bufio.NewReader(strings.NewReader("\n  hello  \n/clear\n/exit\n")),
		out:    io.Discard,
		errOut: io.Discard,
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
}

func TestLineChatLoopPropagatesForceExit(t *testing.T) {
	t.Parallel()

	loop := &lineChatLoop{
		reader: bufio.NewReader(strings.NewReader("run\n")),
		out:    io.Discard,
		errOut: io.Discard,
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
