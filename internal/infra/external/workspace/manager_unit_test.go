package workspace

import (
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestBranchName(t *testing.T) {
	tests := []struct {
		taskID string
		want   string
	}{
		{"task-1", "elephant/task-1"},
		{"my task", "elephant/my-task"},
		{"path/slash", "elephant/path-slash"},
		{"back\\slash", "elephant/back-slash"},
		{"  padded  ", "elephant/padded"},
		{"", "elephant/task"},
	}
	for _, tt := range tests {
		t.Run(tt.taskID, func(t *testing.T) {
			if got := branchName(tt.taskID); got != tt.want {
				t.Errorf("branchName(%q) = %q, want %q", tt.taskID, got, tt.want)
			}
		})
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"single", "hello", 1},
		{"multi", "a\nb\nc", 3},
		{"trailing newline", "a\nb\n", 2},
		{"blank lines filtered", "a\n\nb\n\n", 2},
		{"whitespace only lines", "  \n  \n", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitLines(%q) len = %d, want %d, got %v", tt.input, len(got), tt.want, got)
			}
		})
	}
}

func TestWithinScope(t *testing.T) {
	tests := []struct {
		path  string
		scope []string
		want  bool
	}{
		{"internal/agent/foo.go", []string{"internal/agent/"}, true},
		{"internal/agent/foo.go", []string{"internal/infra/"}, false},
		{"internal/agent/foo.go", []string{"internal/infra/", "internal/agent/"}, true},
		{"other.go", []string{""}, false},
		{"file.go", nil, false},
		{"file.go", []string{"  "}, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := withinScope(tt.path, tt.scope); got != tt.want {
				t.Errorf("withinScope(%q, %v) = %v, want %v", tt.path, tt.scope, got, tt.want)
			}
		})
	}
}

func TestOverlapPaths(t *testing.T) {
	tests := []struct {
		name string
		a, b []string
		want int
	}{
		{"no overlap", []string{"a/"}, []string{"b/"}, 0},
		{"prefix match a->b", []string{"internal/agent/foo.go"}, []string{"internal/agent/"}, 1},
		{"prefix match b->a", []string{"internal/"}, []string{"internal/agent/foo.go"}, 1},
		{"empty a", nil, []string{"a/"}, 0},
		{"empty b", []string{"a/"}, nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overlapPaths(tt.a, tt.b)
			if len(got) != tt.want {
				t.Errorf("overlapPaths() len = %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestDedupe(t *testing.T) {
	got := dedupe([]string{"a", "b", "a", "c", "b"})
	if len(got) != 3 {
		t.Fatalf("dedupe len = %d, want 3", len(got))
	}
}

func TestDedupe_Nil(t *testing.T) {
	got := dedupe(nil)
	if got != nil {
		t.Errorf("dedupe(nil) = %v, want nil", got)
	}
}

func TestExtractUnmergedPaths(t *testing.T) {
	input := "100644 abc123 1\tfile1.go\n100644 def456 2\tfile1.go\n100644 ghi789 3\tfile2.go\n"
	got := extractUnmergedPaths(input)
	if len(got) != 2 {
		t.Fatalf("extractUnmergedPaths len = %d, want 2, got %v", len(got), got)
	}
}

func TestExtractUnmergedPaths_Empty(t *testing.T) {
	got := extractUnmergedPaths("")
	if len(got) != 0 {
		t.Errorf("extractUnmergedPaths('') = %v, want empty", got)
	}
}

func TestCheckScopeOverlap(t *testing.T) {
	m := NewManager("/tmp/test", nil)

	t.Run("no new scope", func(t *testing.T) {
		conflicts := m.CheckScopeOverlap(nil, []agent.BackgroundTaskSummary{
			{ID: "t1", FileScope: []string{"internal/"}},
		})
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts, got %d", len(conflicts))
		}
	})

	t.Run("no running tasks", func(t *testing.T) {
		conflicts := m.CheckScopeOverlap([]string{"internal/"}, nil)
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts, got %d", len(conflicts))
		}
	})

	t.Run("overlap detected", func(t *testing.T) {
		conflicts := m.CheckScopeOverlap(
			[]string{"internal/agent/foo.go"},
			[]agent.BackgroundTaskSummary{
				{ID: "t1", FileScope: []string{"internal/agent/"}},
				{ID: "t2", FileScope: []string{"internal/infra/"}},
			},
		)
		if len(conflicts) != 1 {
			t.Fatalf("expected 1 conflict, got %d", len(conflicts))
		}
		if conflicts[0].TaskID != "t1" {
			t.Errorf("TaskID = %q, want %q", conflicts[0].TaskID, "t1")
		}
	})

	t.Run("running task with no scope", func(t *testing.T) {
		conflicts := m.CheckScopeOverlap(
			[]string{"internal/"},
			[]agent.BackgroundTaskSummary{
				{ID: "t1"},
			},
		)
		if len(conflicts) != 0 {
			t.Errorf("expected 0 conflicts, got %d", len(conflicts))
		}
	})
}
