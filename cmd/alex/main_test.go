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
