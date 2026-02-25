package main

import "testing"

func TestParseUserCommand(t *testing.T) {
	cases := []struct {
		name  string
		input string
		kind  userCommandKind
		task  string
	}{
		{name: "empty", input: "", kind: commandEmpty},
		{name: "whitespace", input: "  ", kind: commandEmpty},
		{name: "quit", input: "/quit", kind: commandQuit},
		{name: "exit", input: "/exit", kind: commandQuit},
		{name: "clear", input: "/clear", kind: commandClear},
		{name: "help", input: "/help", kind: commandHelp},
		{name: "help short", input: "/?", kind: commandHelp},
		{name: "task trimmed", input: "  hello  ", kind: commandRun, task: "hello"},
		{name: "command as task", input: "/unknown", kind: commandRun, task: "/unknown"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := parseUserCommand(tc.input)
			if cmd.kind != tc.kind {
				t.Fatalf("expected kind %v, got %v", tc.kind, cmd.kind)
			}
			if cmd.task != tc.task {
				t.Fatalf("expected task %q, got %q", tc.task, cmd.task)
			}
		})
	}
}
