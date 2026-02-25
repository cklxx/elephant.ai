package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"testing"

	"alex/internal/app/di"
	sessionstate "alex/internal/infra/session/state_store"
)

func TestFormatBufferedFlagParseErrorMatchesLegacyFormatting(t *testing.T) {
	t.Parallel()

	fs := flag.NewFlagSet("legacy", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)
	fs.Bool("known", false, "known flag")

	err := fs.Parse([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}

	want := fmt.Sprintf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	got := formatBufferedFlagParseError(err, &flagBuf).Error()
	if got != want {
		t.Fatalf("formatted parse error mismatch:\nwant: %q\ngot:  %q", want, got)
	}
}

func TestHandleEvalFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	var c CLI
	err := c.handleEval([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("expected unknown-flag message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage of eval:") {
		t.Fatalf("expected eval usage in error, got %q", err.Error())
	}
}

func TestRunFoundationEvaluationFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	var c CLI
	err := c.runFoundationEvaluation([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("expected unknown-flag message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage of eval foundation:") {
		t.Fatalf("expected foundation usage in error, got %q", err.Error())
	}
}

func TestRunFoundationSuiteEvaluationFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	var c CLI
	err := c.runFoundationSuiteEvaluation([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("expected unknown-flag message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage of eval foundation-suite:") {
		t.Fatalf("expected foundation-suite usage in error, got %q", err.Error())
	}
}

func TestPullSessionSnapshotsWithWriterFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	store := sessionstate.NewInMemoryStore()
	cli := &CLI{container: &Container{Container: &di.Container{StateStore: store}}}
	var out bytes.Buffer

	err := cli.pullSessionSnapshotsWithWriter(context.Background(), []string{"sess-1", "--unknown"}, &out)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("expected unknown-flag message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "Usage of sessions pull:") {
		t.Fatalf("expected sessions pull usage in error, got %q", err.Error())
	}
}

func TestRunLarkScenarioRunFlagParseErrorUsesExitCode2(t *testing.T) {
	t.Parallel()

	err := runLarkScenarioRun([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
	if !strings.Contains(exitErr.Err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("expected unknown-flag message, got %q", exitErr.Err.Error())
	}
	if !strings.Contains(exitErr.Err.Error(), "Usage of alex lark scenario run:") {
		t.Fatalf("expected scenario usage in error, got %q", exitErr.Err.Error())
	}
}

func TestRunLarkInjectCommandFlagParseErrorUsesExitCode2(t *testing.T) {
	t.Parallel()

	err := runLarkInjectCommand([]string{"--unknown"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
	if !strings.Contains(exitErr.Err.Error(), "flag provided but not defined: -unknown") {
		t.Fatalf("expected unknown-flag message, got %q", exitErr.Err.Error())
	}
	if !strings.Contains(exitErr.Err.Error(), "Usage of alex lark inject:") {
		t.Fatalf("expected inject usage in error, got %q", exitErr.Err.Error())
	}
}
