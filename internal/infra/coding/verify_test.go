package coding

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type stubCommandRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   []string
}

func (s *stubCommandRunner) Run(_ context.Context, _ string, command string) (string, error) {
	s.calls = append(s.calls, command)
	if err, ok := s.errs[command]; ok {
		return s.outputs[command], err
	}
	return s.outputs[command], nil
}

func TestResolveVerificationPlan(t *testing.T) {
	if plan := ResolveVerificationPlan(nil); plan.Enabled {
		t.Fatalf("expected disabled plan for nil config, got %+v", plan)
	}

	plan := ResolveVerificationPlan(map[string]string{"verify": "true"})
	if !plan.Enabled {
		t.Fatal("expected verification enabled")
	}
	if plan.Build != defaultVerifyBuild || plan.Test != defaultVerifyTest || plan.Lint != defaultVerifyLint {
		t.Fatalf("expected default commands, got %+v", plan)
	}

	plan = ResolveVerificationPlan(map[string]string{
		"verify":           "yes",
		"verify_build_cmd": "make build",
		"verify_test_cmd":  "make test",
		"verify_lint_cmd":  "make lint",
	})
	if plan.Build != "make build" || plan.Test != "make test" || plan.Lint != "make lint" {
		t.Fatalf("expected custom commands, got %+v", plan)
	}
}

func TestVerifyAll(t *testing.T) {
	runner := &stubCommandRunner{
		outputs: map[string]string{
			"make build": "ok build",
			"make test":  "ok test",
		},
		errs: map[string]error{
			"make lint": errors.New("lint failed"),
		},
	}

	result := VerifyAll(context.Background(), ".", runner, VerificationPlan{
		Enabled: true,
		Build:   "make build",
		Test:    "make test",
		Lint:    "make lint",
	})
	if result == nil {
		t.Fatal("expected verification result")
	}
	if result.Passed {
		t.Fatalf("expected failed result, got %+v", result)
	}
	if len(result.Checks) != 3 {
		t.Fatalf("expected 3 checks, got %d", len(result.Checks))
	}
	if err := VerifyError(result); err == nil {
		t.Fatal("expected verify error for failed checks")
	}
}

func TestVerifyAll_SkipsEmptyCommands(t *testing.T) {
	runner := &stubCommandRunner{outputs: map[string]string{}}
	result := VerifyAll(context.Background(), ".", runner, VerificationPlan{
		Enabled: true,
		Test:    "go test ./...",
	})
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if !result.Passed {
		t.Fatalf("expected pass when executed check succeeds, got %+v", result)
	}
	if !result.Checks[0].Skipped || result.Checks[1].Skipped || !result.Checks[2].Skipped {
		t.Fatalf("unexpected skipped flags: %+v", result.Checks)
	}
}

func TestShellCommandRunnerRunsWithoutShellInterpretation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("command expectations in this test are unix-specific")
	}

	runner := shellCommandRunner{}
	dir := t.TempDir()
	out, err := runner.Run(context.Background(), dir, `python3 -c "print('verify-ok')"`)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.TrimSpace(out) != "verify-ok" {
		t.Fatalf("expected verify-ok output, got %q", out)
	}
}

func TestShellCommandRunnerRejectsShellSyntaxInjection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("path assumptions in this test are unix-specific")
	}

	runner := shellCommandRunner{}
	dir := t.TempDir()
	marker := filepath.Join(dir, "marker.txt")
	command := `python3 -c "print('verify-ok')"; touch ` + marker

	_, err := runner.Run(context.Background(), dir, command)
	if err == nil {
		t.Fatal("expected shell syntax injection to be rejected")
	}
	if !strings.Contains(err.Error(), "unsupported shell syntax") {
		t.Fatalf("expected unsupported shell syntax error, got %v", err)
	}
	if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
		t.Fatalf("expected marker to not be created, stat err=%v", statErr)
	}
}

func TestShellCommandRunnerRejectsShellExecutable(t *testing.T) {
	runner := shellCommandRunner{}
	_, err := runner.Run(context.Background(), t.TempDir(), `bash -lc "echo owned"`)
	if err == nil {
		t.Fatal("expected shell executable to be rejected")
	}
	if !strings.Contains(err.Error(), "shell interpreter") {
		t.Fatalf("expected shell interpreter rejection, got %v", err)
	}
}

func TestSplitCommandArgsPreservesQuotedArguments(t *testing.T) {
	args, err := splitCommandArgs(`go test -run "Test Verify" ./...`)
	if err != nil {
		t.Fatalf("splitCommandArgs() error = %v", err)
	}
	want := []string{"go", "test", "-run", "Test Verify", "./..."}
	if len(args) != len(want) {
		t.Fatalf("splitCommandArgs() len=%d, want %d (%v)", len(args), len(want), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("splitCommandArgs()[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}
