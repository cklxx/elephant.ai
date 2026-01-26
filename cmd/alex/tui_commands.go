package main

import "strings"

type userCommandKind int

const (
	commandUnknown userCommandKind = iota
	commandEmpty
	commandQuit
	commandClear
	commandRun
)

type userCommand struct {
	kind userCommandKind
	task string
}

func parseUserCommand(input string) userCommand {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return userCommand{kind: commandEmpty}
	}

	switch trimmed {
	case "/quit", "/exit":
		return userCommand{kind: commandQuit}
	case "/clear":
		return userCommand{kind: commandClear}
	default:
		return userCommand{kind: commandRun, task: trimmed}
	}
}
