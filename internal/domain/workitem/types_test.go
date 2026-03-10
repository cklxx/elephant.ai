package workitem

import "testing"

func TestStatusClass_IsTerminal(t *testing.T) {
	tests := []struct {
		status StatusClass
		want   bool
	}{
		{StatusTodo, false},
		{StatusInProgress, false},
		{StatusBlocked, false},
		{StatusDone, true},
		{StatusCancelled, true},
		{StatusUnknown, false},
	}
	for _, tt := range tests {
		if got := tt.status.IsTerminal(); got != tt.want {
			t.Errorf("StatusClass(%q).IsTerminal() = %v, want %v", tt.status, got, tt.want)
		}
	}
}

func TestProviderConstants(t *testing.T) {
	if ProviderJira != "jira" {
		t.Errorf("ProviderJira = %q, want %q", ProviderJira, "jira")
	}
	if ProviderLinear != "linear" {
		t.Errorf("ProviderLinear = %q, want %q", ProviderLinear, "linear")
	}
}

func TestStatusClassConstants(t *testing.T) {
	all := []StatusClass{
		StatusTodo, StatusInProgress, StatusBlocked,
		StatusDone, StatusCancelled, StatusUnknown,
	}
	seen := make(map[StatusClass]bool)
	for _, s := range all {
		if seen[s] {
			t.Errorf("duplicate StatusClass constant: %q", s)
		}
		seen[s] = true
		if s == "" {
			t.Error("StatusClass constant must not be empty")
		}
	}
}
