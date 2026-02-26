package lark

import "testing"

func TestNormalizeTaskStatus(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "pending", input: "pending", expect: taskStatusPending},
		{name: "running", input: "running", expect: taskStatusRunning},
		{name: "waiting", input: "waiting_input", expect: taskStatusWaitingInput},
		{name: "completed uppercase", input: "COMPLETED", expect: taskStatusCompleted},
		{name: "success alias", input: " success ", expect: taskStatusCompleted},
		{name: "failed alias", input: "error", expect: taskStatusFailed},
		{name: "cancelled alias", input: "canceled", expect: taskStatusCancelled},
		{name: "unknown passthrough", input: "paused", expect: "paused"},
		{name: "empty", input: "", expect: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeTaskStatus(tt.input)
			if got != tt.expect {
				t.Fatalf("normalizeTaskStatus(%q) = %q, want %q", tt.input, got, tt.expect)
			}
		})
	}
}

func TestNormalizeCompletionTaskStatus(t *testing.T) {
	tests := []struct {
		name   string
		status string
		err    string
		expect string
	}{
		{name: "keep terminal completed", status: "completed", expect: taskStatusCompleted},
		{name: "alias done", status: "done", expect: taskStatusCompleted},
		{name: "running with error becomes failed", status: "running", err: "boom", expect: taskStatusFailed},
		{name: "running without error becomes completed", status: "running", expect: taskStatusCompleted},
		{name: "empty with error becomes failed", status: "", err: "timeout", expect: taskStatusFailed},
		{name: "empty without error becomes completed", status: "", expect: taskStatusCompleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeCompletionTaskStatus(tt.status, tt.err)
			if got != tt.expect {
				t.Fatalf("normalizeCompletionTaskStatus(%q, %q) = %q, want %q", tt.status, tt.err, got, tt.expect)
			}
		})
	}
}
