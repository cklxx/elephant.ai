package main

import (
	"strings"
	"testing"
)

func TestCLIHandleConfigAcceptsShow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	cli := &CLI{}
	if err := cli.handleConfig([]string{"show"}); err != nil {
		t.Fatalf("handleConfig returned error for show: %v", err)
	}
}

func TestCLIHandleConfigRejectsUnknownSubcommand(t *testing.T) {
	cli := &CLI{}
	err := cli.handleConfig([]string{"unknown"})
	if err == nil {
		t.Fatalf("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown config subcommand") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCLIHandleConfigRejectsTooManyArgs(t *testing.T) {
	cli := &CLI{}
	err := cli.handleConfig([]string{"show", "extra"})
	if err == nil {
		t.Fatalf("expected error for too many arguments")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Fatalf("unexpected error: %v", err)
	}
}
