package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"alex/internal/app/di"
	sessionstate "alex/internal/infra/session/state_store"
)

func TestFormatBufferedFlagParseErrorMatchesLegacyFormatting(t *testing.T) {
	t.Parallel()

	err := errors.New("flag parse failed")
	var flagBuf bytes.Buffer
	flagBuf.WriteString("usage line\n")

	formatted := formatBufferedFlagParseError(err, &flagBuf)
	if formatted == nil {
		t.Fatalf("expected formatted error")
	}
	if got := formatted.Error(); got != "flag parse failed: usage line" {
		t.Fatalf("unexpected formatted error: %q", got)
	}
}

func TestParseBufferedFlagSetHelpIncludesUsageOutput(t *testing.T) {
	t.Parallel()

	fs, flagBuf := newBufferedFlagSet("test")
	fs.Bool("verbose", false, "Enable verbose logging")

	err := parseBufferedFlagSet(fs, flagBuf, []string{"-h"})
	if err == nil {
		t.Fatalf("expected help parse error")
	}
	if !strings.Contains(err.Error(), "flag: help requested") {
		t.Fatalf("unexpected help parse error: %v", err)
	}
	if !strings.Contains(err.Error(), "-verbose") {
		t.Fatalf("expected help output to include defined flag, got: %v", err)
	}
}

func TestHandleEvalFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	var cli CLI
	err := cli.handleEval([]string{"--no-such-flag"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestRunFoundationEvaluationFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	var cli CLI
	err := cli.runFoundationEvaluation([]string{"--bad-flag"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestRunFoundationSuiteEvaluationFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	var cli CLI
	err := cli.runFoundationSuiteEvaluation([]string{"--bad-flag"})
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestPullSessionSnapshotsWithWriterFlagParseErrorIncludesBufferedUsage(t *testing.T) {
	t.Parallel()

	store := sessionstate.NewInMemoryStore()
	cli := &CLI{container: &Container{Container: &di.Container{StateStore: store}}}
	var out bytes.Buffer

	err := cli.pullSessionSnapshotsWithWriter(context.Background(), []string{"sess-1", "--bad-flag"}, &out)
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if !strings.Contains(err.Error(), "flag provided but not defined") {
		t.Fatalf("unexpected parse error: %v", err)
	}
}

func TestRunLarkScenarioRunFlagParseErrorUsesExitCode2(t *testing.T) {
	t.Parallel()

	err := runLarkScenarioRun([]string{"--bad-flag"})
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr == nil {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
	if !strings.Contains(fmt.Sprint(exitErr.Err), "flag provided but not defined") {
		t.Fatalf("unexpected parse error: %v", exitErr.Err)
	}
}

func TestRunLarkInjectCommandFlagParseErrorUsesExitCode2(t *testing.T) {
	t.Parallel()

	err := runLarkInjectCommand([]string{"--bad-flag"})
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) || exitErr == nil {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
	}
	if !strings.Contains(fmt.Sprint(exitErr.Err), "flag provided but not defined") {
		t.Fatalf("unexpected parse error: %v", exitErr.Err)
	}
}
