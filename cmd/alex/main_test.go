package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestCLIExitBehaviorFromError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantCode  int
		wantPrint bool
	}{
		{
			name:      "force exit",
			err:       ErrForceExit,
			wantCode:  130,
			wantPrint: false,
		},
		{
			name:      "wrapped force exit",
			err:       fmt.Errorf("wrapped: %w", ErrForceExit),
			wantCode:  130,
			wantPrint: false,
		},
		{
			name:      "exit code error",
			err:       &ExitCodeError{Code: 7, Err: errors.New("boom")},
			wantCode:  7,
			wantPrint: true,
		},
		{
			name:      "wrapped exit code error",
			err:       fmt.Errorf("wrapped: %w", &ExitCodeError{Code: 2, Err: errors.New("boom")}),
			wantCode:  2,
			wantPrint: true,
		},
		{
			name:      "zero code exit error falls back to one",
			err:       &ExitCodeError{Code: 0, Err: errors.New("boom")},
			wantCode:  1,
			wantPrint: true,
		},
		{
			name:      "generic error",
			err:       errors.New("boom"),
			wantCode:  1,
			wantPrint: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotCode, gotPrint := cliExitBehaviorFromError(tc.err)
			if gotCode != tc.wantCode {
				t.Fatalf("expected exit code %d, got %d", tc.wantCode, gotCode)
			}
			if gotPrint != tc.wantPrint {
				t.Fatalf("expected print=%v, got %v", tc.wantPrint, gotPrint)
			}
		})
	}
}

func TestRunRegisteredCommand_AllowsContainerlessCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "leader help",
			args: []string{"leader", "help"},
		},
		{
			name: "runtime help",
			args: []string{"runtime", "help"},
		},
		{
			name: "dev help",
			args: []string{"dev", "help"},
		},
		{
			name: "model help",
			args: []string{"model", "help"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			handled, err := NewCLI(nil).runRegisteredCommand(tc.args)
			if !handled {
				t.Fatal("expected command to be handled")
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunRegisteredCommand_LeavesContainerCommandsForMainPath(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"sessions"},
		{"cost", "show"},
		{"resume", "session-123"},
		{"acp"},
	} {
		handled, err := NewCLI(nil).runRegisteredCommand(args)
		if handled {
			t.Fatalf("expected %v to be deferred for container initialization", args)
		}
		if err != nil {
			t.Fatalf("expected nil error for deferred command %v, got %v", args, err)
		}
	}
}

func TestIsTopLevelHelp(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want bool
	}{
		{
			name: "empty args",
			args: nil,
			want: false,
		},
		{
			name: "top-level help command",
			args: []string{"help"},
			want: true,
		},
		{
			name: "top-level short help flag",
			args: []string{"-h"},
			want: true,
		},
		{
			name: "top-level long help flag",
			args: []string{"--help"},
			want: true,
		},
		{
			name: "nested help flag should not be treated as top-level help",
			args: []string{"dev", "lark", "--help"},
			want: false,
		},
		{
			name: "unknown command with help flag should not be treated as top-level help",
			args: []string{"unknown", "--help"},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := isTopLevelHelp(tc.args)
			if got != tc.want {
				t.Fatalf("isTopLevelHelp(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}
