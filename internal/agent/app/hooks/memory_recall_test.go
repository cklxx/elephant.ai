package hooks

import (
	"context"
	"errors"
	"testing"

	"alex/internal/memory"
)

// mockMemoryService implements memory.Service for testing.
type mockMemoryService struct {
	recallResult []memory.Entry
	recallErr    error
	recallCalled int
	lastQuery    memory.Query

	saveResult memory.Entry
	saveErr    error
	saveCalled int
	lastEntry  memory.Entry
	entries    []memory.Entry
}

func (m *mockMemoryService) Recall(_ context.Context, query memory.Query) ([]memory.Entry, error) {
	m.recallCalled++
	m.lastQuery = query
	return m.recallResult, m.recallErr
}

func (m *mockMemoryService) Save(_ context.Context, entry memory.Entry) (memory.Entry, error) {
	m.saveCalled++
	m.lastEntry = entry
	m.entries = append(m.entries, entry)
	return m.saveResult, m.saveErr
}

func TestMemoryRecallHook_Name(t *testing.T) {
	hook := NewMemoryRecallHook(nil, nil, MemoryRecallConfig{Enabled: true, AutoRecall: true})
	if hook.Name() != "memory_recall" {
		t.Errorf("expected name 'memory_recall', got %q", hook.Name())
	}
}

func TestMemoryRecallHook_OnTaskStart_NilService(t *testing.T) {
	hook := NewMemoryRecallHook(nil, nil, MemoryRecallConfig{Enabled: true, AutoRecall: true})
	result := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: "test"})
	if result != nil {
		t.Errorf("expected nil injections with nil service, got %v", result)
	}
}

func TestMemoryRecallHook_OnTaskStart_EmptyInput(t *testing.T) {
	svc := &mockMemoryService{}
	hook := NewMemoryRecallHook(svc, nil, MemoryRecallConfig{Enabled: true, AutoRecall: true})

	result := hook.OnTaskStart(context.Background(), TaskInfo{TaskInput: ""})
	if result != nil {
		t.Errorf("expected nil injections for empty input, got %v", result)
	}
	if svc.recallCalled != 0 {
		t.Errorf("expected no recall call for empty input, got %d", svc.recallCalled)
	}
}

func TestMemoryRecallHook_OnTaskStart_SuccessfulRecall(t *testing.T) {
	svc := &mockMemoryService{
		recallResult: []memory.Entry{
			{Key: "1", Content: "Previous discussion about deployment", Keywords: []string{"deployment", "ci"}},
			{Key: "2", Content: "Database migration plan", Keywords: []string{"database", "migration"}},
		},
	}
	hook := NewMemoryRecallHook(svc, nil, MemoryRecallConfig{Enabled: true, AutoRecall: true, MaxRecalls: 3})

	result := hook.OnTaskStart(context.Background(), TaskInfo{
		TaskInput: "deploy the new database migration",
		UserID:    "testuser",
	})

	if svc.recallCalled != 1 {
		t.Fatalf("expected 1 recall call, got %d", svc.recallCalled)
	}
	if svc.lastQuery.UserID != "testuser" {
		t.Errorf("expected userID 'testuser', got %q", svc.lastQuery.UserID)
	}
	if svc.lastQuery.Text != "deploy the new database migration" {
		t.Errorf("expected query text to match task input, got %q", svc.lastQuery.Text)
	}
	if svc.lastQuery.Limit != 3 {
		t.Errorf("expected limit 3, got %d", svc.lastQuery.Limit)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(result))
	}
	if result[0].Type != InjectionMemoryRecall {
		t.Errorf("expected type MemoryRecall, got %v", result[0].Type)
	}
	if result[0].Source != "memory_recall" {
		t.Errorf("expected source 'memory_recall', got %q", result[0].Source)
	}
	if result[0].Priority != 100 {
		t.Errorf("expected priority 100, got %d", result[0].Priority)
	}

	// Verify content contains memory entries
	if !contains(result[0].Content, "deployment") {
		t.Error("expected content to contain 'deployment'")
	}
	if !contains(result[0].Content, "Database migration plan") {
		t.Error("expected content to contain 'Database migration plan'")
	}
}

func TestMemoryRecallHook_OnTaskStart_NoResults(t *testing.T) {
	svc := &mockMemoryService{recallResult: []memory.Entry{}}
	hook := NewMemoryRecallHook(svc, nil, MemoryRecallConfig{Enabled: true, AutoRecall: true})

	result := hook.OnTaskStart(context.Background(), TaskInfo{
		TaskInput: "test query",
		UserID:    "testuser",
	})

	if result != nil {
		t.Errorf("expected nil injections for empty results, got %v", result)
	}
}

func TestMemoryRecallHook_OnTaskStart_RecallError(t *testing.T) {
	svc := &mockMemoryService{recallErr: errors.New("store unavailable")}
	hook := NewMemoryRecallHook(svc, nil, MemoryRecallConfig{Enabled: true, AutoRecall: true})

	result := hook.OnTaskStart(context.Background(), TaskInfo{
		TaskInput: "test query",
		UserID:    "testuser",
	})

	if result != nil {
		t.Errorf("expected nil injections on error, got %v", result)
	}
}

func TestMemoryRecallHook_OnTaskStart_DefaultUserID(t *testing.T) {
	svc := &mockMemoryService{recallResult: []memory.Entry{
		{Key: "1", Content: "test memory"},
	}}
	hook := NewMemoryRecallHook(svc, nil, MemoryRecallConfig{Enabled: true, AutoRecall: true})

	hook.OnTaskStart(context.Background(), TaskInfo{
		TaskInput: "some query",
		UserID:    "", // empty user ID
	})

	if svc.lastQuery.UserID != "default" {
		t.Errorf("expected default userID 'default', got %q", svc.lastQuery.UserID)
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantMin  int
		wantMax  int
		contains []string
		excludes []string
	}{
		{
			name:     "basic English",
			input:    "deploy the new database migration",
			contains: []string{"deploy", "new", "database", "migration"},
			excludes: []string{"the"},
		},
		{
			name:     "Chinese text",
			input:    "部署新的数据库迁移方案",
			wantMin:  1,
			contains: []string{},
		},
		{
			name:    "empty input",
			input:   "",
			wantMax: 0,
		},
		{
			name:     "stop words filtered",
			input:    "please help me with the deployment",
			contains: []string{"deployment"},
			excludes: []string{"please", "help", "me", "with", "the"},
		},
		{
			name:     "short words filtered",
			input:    "I a b c deploy",
			contains: []string{"deploy"},
			excludes: []string{"a", "b", "c"},
		},
		{
			name:    "duplicates removed",
			input:   "deploy deploy deploy migration",
			wantMax: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keywords := extractKeywords(tt.input)

			if tt.wantMax > 0 && len(keywords) > tt.wantMax {
				t.Errorf("expected at most %d keywords, got %d: %v", tt.wantMax, len(keywords), keywords)
			}

			for _, want := range tt.contains {
				found := false
				for _, kw := range keywords {
					if kw == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected keywords to contain %q, got %v", want, keywords)
				}
			}

			for _, exclude := range tt.excludes {
				for _, kw := range keywords {
					if kw == exclude {
						t.Errorf("expected keywords NOT to contain %q, got %v", exclude, keywords)
						break
					}
				}
			}
		})
	}
}

func TestExtractKeywords_MaxCap(t *testing.T) {
	// Generate a long input with many distinct words
	words := make([]string, 20)
	for i := range words {
		words[i] = "keyword" + string(rune('a'+i))
	}
	input := ""
	for _, w := range words {
		input += w + " "
	}

	keywords := extractKeywords(input)
	if len(keywords) > 10 {
		t.Errorf("expected at most 10 keywords, got %d", len(keywords))
	}
}

func TestFormatMemoryEntries(t *testing.T) {
	entries := []memory.Entry{
		{Content: "First memory", Keywords: []string{"alpha", "beta"}},
		{Content: "Second memory", Keywords: nil},
	}

	result := formatMemoryEntries(entries)

	if !contains(result, "Memory 1") {
		t.Error("expected 'Memory 1' in output")
	}
	if !contains(result, "alpha, beta") {
		t.Error("expected keywords in output")
	}
	if !contains(result, "First memory") {
		t.Error("expected first memory content")
	}
	if !contains(result, "Memory 2") {
		t.Error("expected 'Memory 2' in output")
	}
	if !contains(result, "Second memory") {
		t.Error("expected second memory content")
	}
}
