package ports

import "testing"

func TestTaskStatusIsTerminal(t *testing.T) {
	tests := []struct {
		name   string
		status TaskStatus
		want   bool
	}{
		{name: "pending", status: TaskStatusPending, want: false},
		{name: "running", status: TaskStatusRunning, want: false},
		{name: "waiting input", status: TaskStatusWaitingInput, want: false},
		{name: "completed", status: TaskStatusCompleted, want: true},
		{name: "failed", status: TaskStatusFailed, want: true},
		{name: "cancelled", status: TaskStatusCancelled, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.want {
				t.Fatalf("IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}
