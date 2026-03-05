package main

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
	id "alex/internal/shared/utils/id"
)

func TestParseTeamRunOptions(t *testing.T) {
	t.Run("requires file or template", func(t *testing.T) {
		_, err := parseTeamRunOptions(nil)
		if err == nil || !strings.Contains(err.Error(), teamRunUsage) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})

	t.Run("rejects mutually exclusive sources", func(t *testing.T) {
		_, err := parseTeamRunOptions([]string{"--file", "tasks.yaml", "--template", "demo", "--goal", "x"})
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("expected mutual exclusion error, got %v", err)
		}
	})

	t.Run("template requires goal", func(t *testing.T) {
		_, err := parseTeamRunOptions([]string{"--template", "demo"})
		if err == nil || !strings.Contains(err.Error(), "--goal is required") {
			t.Fatalf("expected goal-required error, got %v", err)
		}
	})

	t.Run("file mode rejects template-only flags", func(t *testing.T) {
		_, err := parseTeamRunOptions([]string{"--file", "tasks.yaml", "--goal", "x"})
		if err == nil || !strings.Contains(err.Error(), "--goal can only be used with --template") {
			t.Fatalf("expected goal mode error, got %v", err)
		}

		_, err = parseTeamRunOptions([]string{"--file", "tasks.yaml", "--role-prompt", "planner=focus"})
		if err == nil || !strings.Contains(err.Error(), "--role-prompt can only be used with --template") {
			t.Fatalf("expected role-prompt mode error, got %v", err)
		}
	})

	t.Run("wait timeout requires wait", func(t *testing.T) {
		_, err := parseTeamRunOptions([]string{"--file", "tasks.yaml", "--wait-timeout-seconds", "200"})
		if err == nil || !strings.Contains(err.Error(), "requires --wait") {
			t.Fatalf("expected wait-timeout gate, got %v", err)
		}
	})

	t.Run("validates mode", func(t *testing.T) {
		_, err := parseTeamRunOptions([]string{"--file", "tasks.yaml", "--mode", "parallel"})
		if err == nil || !strings.Contains(err.Error(), "invalid --mode") {
			t.Fatalf("expected invalid mode error, got %v", err)
		}
	})

	t.Run("parses file mode", func(t *testing.T) {
		opts, err := parseTeamRunOptions([]string{
			"--file", "tasks.yaml",
			"--mode", "swarm",
			"--only-task", "impl,test",
			"--only-task", "lint",
		})
		if err != nil {
			t.Fatalf("parseTeamRunOptions returned error: %v", err)
		}
		if opts.filePath != "tasks.yaml" || opts.templateName != "" {
			t.Fatalf("unexpected source selection: %+v", opts)
		}
		if opts.mode != "swarm" || opts.wait {
			t.Fatalf("unexpected mode/wait values: %+v", opts)
		}
		if opts.waitTimeoutSec != defaultTeamWaitTimeoutInSec {
			t.Fatalf("unexpected default wait timeout: %d", opts.waitTimeoutSec)
		}
		wantIDs := []string{"impl", "test", "lint"}
		if !reflect.DeepEqual(opts.onlyTaskIDs, wantIDs) {
			t.Fatalf("onlyTaskIDs = %#v, want %#v", opts.onlyTaskIDs, wantIDs)
		}
	})

	t.Run("parses template mode", func(t *testing.T) {
		opts, err := parseTeamRunOptions([]string{
			"--template", "execute_and_report",
			"--goal", "review auth module",
			"--wait",
			"--wait-timeout-seconds", "180",
			"--session-id", "sess-1",
			"--role-prompt", "research=focus on consistency",
			"--role-prompt", "executor=produce runnable patch",
		})
		if err != nil {
			t.Fatalf("parseTeamRunOptions returned error: %v", err)
		}
		if opts.templateName != "execute_and_report" || opts.goalText != "review auth module" {
			t.Fatalf("unexpected template/goal: %+v", opts)
		}
		if !opts.wait || opts.waitTimeoutSec != 180 {
			t.Fatalf("unexpected wait config: %+v", opts)
		}
		if opts.sessionID != "sess-1" {
			t.Fatalf("sessionID = %q, want sess-1", opts.sessionID)
		}
		if len(opts.rolePromptByID) != 2 || opts.rolePromptByID["research"] == "" || opts.rolePromptByID["executor"] == "" {
			t.Fatalf("unexpected role prompts: %+v", opts.rolePromptByID)
		}
	})
}

func TestParseTeamReplyOptions(t *testing.T) {
	t.Run("requires task and request ids", func(t *testing.T) {
		_, err := parseTeamReplyOptions([]string{"--task-id", "task-1"})
		if err == nil || !strings.Contains(err.Error(), teamReplyUsage) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})

	t.Run("rejects invalid decision", func(t *testing.T) {
		_, err := parseTeamReplyOptions([]string{
			"--task-id", "task-1",
			"--request-id", "req-1",
			"--decision", "maybe",
		})
		if err == nil || !strings.Contains(err.Error(), "invalid --decision") {
			t.Fatalf("expected decision error, got %v", err)
		}
	})

	t.Run("requires at least one response field", func(t *testing.T) {
		_, err := parseTeamReplyOptions([]string{
			"--task-id", "task-1",
			"--request-id", "req-1",
		})
		if err == nil || !strings.Contains(err.Error(), "at least one of --decision, --option-id, or --message is required") {
			t.Fatalf("expected response payload error, got %v", err)
		}
	})

	t.Run("parses decision and payload", func(t *testing.T) {
		opts, err := parseTeamReplyOptions([]string{
			"--task-id", "task-1",
			"--request-id", "req-1",
			"--decision", "approve",
			"--option-id", "opt-a",
			"--message", "please continue",
		})
		if err != nil {
			t.Fatalf("parseTeamReplyOptions returned error: %v", err)
		}
		if opts.taskID != "task-1" || opts.requestID != "req-1" {
			t.Fatalf("unexpected ids: %+v", opts)
		}
		if opts.decision != "approve" || opts.optionID != "opt-a" || opts.message != "please continue" {
			t.Fatalf("unexpected payload: %+v", opts)
		}
	})
}

func TestParseTeamInjectOptions(t *testing.T) {
	t.Run("requires task and message", func(t *testing.T) {
		_, err := parseTeamInjectOptions([]string{"--task-id", "task-1"})
		if err == nil || !strings.Contains(err.Error(), teamInjectUsage) {
			t.Fatalf("expected usage error, got %v", err)
		}
	})

	t.Run("parses inject payload", func(t *testing.T) {
		opts, err := parseTeamInjectOptions([]string{"--task-id", "task-1", "--message", "continue"})
		if err != nil {
			t.Fatalf("parseTeamInjectOptions returned error: %v", err)
		}
		if opts.taskID != "task-1" || opts.message != "continue" {
			t.Fatalf("unexpected inject opts: %+v", opts)
		}
	})
}

func TestHandleTeamRoutes(t *testing.T) {
	cli := &CLI{}

	t.Run("unknown subcommand", func(t *testing.T) {
		err := cli.handleTeam([]string{"unknown"})
		var exitErr *ExitCodeError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
		}
		if exitErr.Code != 2 {
			t.Fatalf("expected exit code 2, got %d", exitErr.Code)
		}
	})

	t.Run("inject requires container after args parse", func(t *testing.T) {
		err := cli.handleTeam([]string{"inject", "--task-id", "task-1", "--message", "continue"})
		var exitErr *ExitCodeError
		if !errors.As(err, &exitErr) {
			t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
		}
		if exitErr.Code != 1 || !strings.Contains(exitErr.Error(), "container not initialized") {
			t.Fatalf("unexpected error: %+v", exitErr)
		}
	})
}

func TestBuildTeamRunContextAppliesPinnedSelection(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")
	t.Setenv("ALEX_CONFIG_PATH", configPath)

	storePath := subscription.ResolveSelectionStorePath(runtimeconfig.DefaultEnvLookup, nil)
	store := subscription.NewSelectionStore(storePath)
	if err := store.Set(context.Background(), subscription.SelectionScope{Channel: "cli"}, subscription.Selection{
		Mode:     "cli",
		Provider: "llama_server",
		Model:    "llama3:latest",
	}); err != nil {
		t.Fatalf("seed selection store: %v", err)
	}

	ctx := buildTeamRunContext("team-session-1")
	if got := id.SessionIDFromContext(ctx); got != "team-session-1" {
		t.Fatalf("session id mismatch: got %q", got)
	}

	selection, ok := appcontext.GetLLMSelection(ctx)
	if !ok {
		t.Fatalf("expected pinned LLM selection on context")
	}
	if selection.Provider != "llama.cpp" || selection.Model != "llama3:latest" || !selection.Pinned {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}
