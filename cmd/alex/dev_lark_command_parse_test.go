package main

import (
	"strings"
	"testing"
)

func TestParseLarkCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantTarget string
		wantAction string
		wantErr    string
	}{
		{
			name:       "default start",
			args:       nil,
			wantTarget: larkSupervisorTarget,
			wantAction: "start",
		},
		{
			name:       "supervisor command pass through",
			args:       []string{"restart"},
			wantTarget: larkSupervisorTarget,
			wantAction: "restart",
		},
		{
			name:       "component first restart",
			args:       []string{"main", "restart"},
			wantTarget: "main",
			wantAction: "restart",
		},
		{
			name:       "component second restart",
			args:       []string{"restart", "main"},
			wantTarget: "main",
			wantAction: "restart",
		},
		{
			name:       "component second down alias",
			args:       []string{"down", "loop"},
			wantTarget: "loop",
			wantAction: "stop",
		},
		{
			name:    "component missing action",
			args:    []string{"main"},
			wantErr: "missing lark main command",
		},
		{
			name:    "component unknown action",
			args:    []string{"main", "oops"},
			wantErr: "unknown lark main command: oops",
		},
		{
			name:    "component too many args",
			args:    []string{"main", "restart", "extra"},
			wantErr: "too many arguments for lark main command",
		},
		{
			name:       "non-component second arg keeps supervisor parsing",
			args:       []string{"doctor", "main"},
			wantTarget: larkSupervisorTarget,
			wantAction: "doctor",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLarkCommand(tc.args)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("parseLarkCommand(%v) expected error %q, got nil", tc.args, tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("parseLarkCommand(%v) error = %q, want contains %q", tc.args, err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseLarkCommand(%v) returned error: %v", tc.args, err)
			}
			if got.target != tc.wantTarget {
				t.Fatalf("target = %q, want %q", got.target, tc.wantTarget)
			}
			if got.action != tc.wantAction {
				t.Fatalf("action = %q, want %q", got.action, tc.wantAction)
			}
		})
	}
}
