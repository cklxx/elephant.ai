package main

import (
	"errors"
	"fmt"
	"testing"
)

func TestCLIExitBehaviorFromError(t *testing.T) {
	t.Parallel()

	baseErr := errors.New("boom")
	tests := []struct {
		name       string
		err        error
		wantCode   int
		wantStderr bool
	}{
		{
			name:       "force-exit",
			err:        ErrForceExit,
			wantCode:   130,
			wantStderr: false,
		},
		{
			name:       "wrapped-force-exit",
			err:        fmt.Errorf("wrapped: %w", ErrForceExit),
			wantCode:   130,
			wantStderr: false,
		},
		{
			name:       "exit-code-error",
			err:        &ExitCodeError{Code: 23, Err: baseErr},
			wantCode:   23,
			wantStderr: true,
		},
		{
			name:       "wrapped-exit-code-error",
			err:        fmt.Errorf("wrapped: %w", &ExitCodeError{Code: 9, Err: baseErr}),
			wantCode:   9,
			wantStderr: true,
		},
		{
			name:       "zero-exit-code-error-falls-back",
			err:        &ExitCodeError{Code: 0, Err: baseErr},
			wantCode:   1,
			wantStderr: true,
		},
		{
			name:       "generic-error",
			err:        baseErr,
			wantCode:   1,
			wantStderr: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotCode, gotStderr := cliExitBehaviorFromError(tc.err)
			if gotCode != tc.wantCode || gotStderr != tc.wantStderr {
				t.Fatalf("unexpected result: code=%d stderr=%v wantCode=%d wantStderr=%v", gotCode, gotStderr, tc.wantCode, tc.wantStderr)
			}
		})
	}
}
