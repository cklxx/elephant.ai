package lark

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestIsTaskCommand(t *testing.T) {
	g := &Gateway{}
	tests := []struct {
		input string
		want  bool
	}{
		{"/cc refactor auth", true},
		{"/cc", true},
		{"/CC refactor", true},
		{"/codex optimize", true},
		{"/codex", true},
		{"/tasks", true},
		{"/task list", true},
		{"/task status abc", true},
		{"/task cancel abc", true},
		{"/task help", true},
		{"/task refactor auth", true},
		{"/model use codex/gpt-5", false},
		{"/reset", false},
		{"hello world", false},
		{"/plan on", false},   // plan is separate
		{"/taskbar", false},   // must not match broader prefix
		{"/tasksfoo", false},  // must not match broader prefix
	}
	for _, tt := range tests {
		got := g.isTaskCommand(tt.input)
		if got != tt.want {
			t.Errorf("isTaskCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBuildDispatchPrompt(t *testing.T) {
	prompt := buildDispatchPrompt("claude_code", "refactor auth module")
	if !strings.Contains(prompt, "agent_type: claude_code") {
		t.Error("prompt should contain agent_type")
	}
	if !strings.Contains(prompt, "refactor auth module") {
		t.Error("prompt should contain description")
	}
	if !strings.Contains(prompt, "workspace_mode: worktree") {
		t.Error("prompt should contain workspace_mode")
	}
	if !strings.Contains(prompt, "<user_task_description>") {
		t.Error("prompt should wrap description in XML delimiters")
	}
	if !strings.Contains(prompt, "</user_task_description>") {
		t.Error("prompt should close XML delimiter")
	}
}

func TestTaskCommandUsage(t *testing.T) {
	usage := taskCommandUsage()
	if !strings.Contains(usage, "/cc") {
		t.Error("usage should mention /cc")
	}
	if !strings.Contains(usage, "/codex") {
		t.Error("usage should mention /codex")
	}
	if !strings.Contains(usage, "/tasks") {
		t.Error("usage should mention /tasks")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m30s"},
		{5 * time.Minute, "5m0s"},
		{65 * time.Minute, "1h5m"},
		{2 * time.Hour, "2h0m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{500, "500"},
		{1000, "1.0k"},
		{1500, "1.5k"},
		{45200, "45.2k"},
	}
	for _, tt := range tests {
		got := formatTokens(tt.n)
		if got != tt.want {
			t.Errorf("formatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestShortID(t *testing.T) {
	if got := shortID("abc"); got != "abc" {
		t.Errorf("shortID(abc) = %q", got)
	}
	if got := shortID("bg-1234567890abcdef"); got != "bg-123456789" {
		t.Errorf("shortID(bg-1234567890abcdef) = %q", got)
	}
}

func TestTaskStatusIcon(t *testing.T) {
	if got := taskStatusLabel("running"); got != "running" {
		t.Errorf("taskStatusLabel(running) = %q", got)
	}
	if got := taskStatusLabel("completed"); got != "done" {
		t.Errorf("taskStatusLabel(completed) = %q", got)
	}
}

func TestAgentShortName(t *testing.T) {
	if got := agentShortName("claude_code"); got != "cc" {
		t.Errorf("agentShortName(claude_code) = %q", got)
	}
	if got := agentShortName("codex"); got != "codex" {
		t.Errorf("agentShortName(codex) = %q", got)
	}
	if got := agentShortName("internal"); got != "task" {
		t.Errorf("agentShortName(internal) = %q", got)
	}
}

func TestFormatTaskDetail(t *testing.T) {
	now := time.Now()
	rec := TaskRecord{
		TaskID:        "bg-abc123",
		AgentType:     "claude_code",
		Status:        "completed",
		Description:   "Refactor auth module",
		CreatedAt:     now.Add(-5 * time.Minute),
		CompletedAt:   now,
		TokensUsed:    12300,
		AnswerPreview: "Done refactoring.",
	}
	detail := formatTaskDetail(rec)
	if !strings.Contains(detail, "bg-abc123") {
		t.Error("detail should contain task ID")
	}
	if !strings.Contains(detail, "claude_code") {
		t.Error("detail should contain agent type")
	}
	if !strings.Contains(detail, "12.3k") {
		t.Error("detail should contain formatted tokens")
	}
	if !strings.Contains(detail, "Done refactoring.") {
		t.Error("detail should contain answer preview")
	}
}

func TestFormatTaskHistory(t *testing.T) {
	now := time.Now()
	tasks := []TaskRecord{
		{
			TaskID:      "bg-111",
			AgentType:   "codex",
			Status:      "completed",
			Description: "Task one",
			CreatedAt:   now.Add(-10 * time.Minute),
			CompletedAt: now.Add(-5 * time.Minute),
			TokensUsed:  5000,
		},
		{
			TaskID:      "bg-222",
			AgentType:   "claude_code",
			Status:      "failed",
			Description: "Task two",
			CreatedAt:   now.Add(-20 * time.Minute),
			CompletedAt: now.Add(-15 * time.Minute),
		},
	}
	history := formatTaskHistory(tasks)
	if !strings.Contains(history, "任务历史") {
		t.Error("history should contain header")
	}
	if !strings.Contains(history, "bg-111") {
		t.Error("history should contain task 1 ID")
	}
	if !strings.Contains(history, "bg-222") {
		t.Error("history should contain task 2 ID")
	}
}

func TestHandleTaskList_NoStore(t *testing.T) {
	g := &Gateway{}
	msg := &incomingMessage{chatID: "chat1"}
	reply := g.handleTaskList(context.Background(), msg)
	if !strings.Contains(reply, "未启用") {
		t.Errorf("expected disabled message, got: %s", reply)
	}
}

func TestHandleTaskStatus_NoStore(t *testing.T) {
	g := &Gateway{}
	reply := g.handleTaskStatus(context.Background(), "task1")
	if !strings.Contains(reply, "未启用") {
		t.Errorf("expected disabled message, got: %s", reply)
	}
}

func TestHandleTaskCancel_NoStore(t *testing.T) {
	g := &Gateway{}
	reply := g.handleTaskCancel(context.Background(), "task1")
	if !strings.Contains(reply, "未启用") {
		t.Errorf("expected disabled message, got: %s", reply)
	}
}
