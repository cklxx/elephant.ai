package kernel

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	toolshared "alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

type stubTaskRunner struct {
	lastPrompt string
	lastCtx    context.Context
	result     *agent.TaskResult
	results    []*agent.TaskResult
	err        error
	prompts    []string
	idx        int
}

func (s *stubTaskRunner) ExecuteTask(ctx context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	s.lastCtx = ctx
	s.lastPrompt = task
	s.prompts = append(s.prompts, task)
	var result *agent.TaskResult
	if s.idx < len(s.results) {
		result = s.results[s.idx]
		s.idx++
	} else {
		result = s.result
	}
	return result, s.err
}

func TestCoordinatorExecutor_AppendsSummaryInstructionAndExtractsSummary(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer:   "前置说明\n## 执行总结\n- 已完成 A\n- 已验证 B",
			Messages: []core.Message{assistantMessageWithToolCalls("read_file")},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	res, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !strings.Contains(runner.lastPrompt, "## 执行总结") {
		t.Fatalf("expected summary instruction appended, got prompt: %q", runner.lastPrompt)
	}
	if !strings.Contains(runner.lastPrompt, "~/.alex/kernel/default/") {
		t.Fatalf("expected kernel write path guidance in prompt, got prompt: %q", runner.lastPrompt)
	}
	if !strings.Contains(res.Summary, "## 执行总结") {
		t.Fatalf("expected summary extracted from answer, got: %q", res.Summary)
	}
	if strings.TrimSpace(res.TaskID) == "" {
		t.Fatalf("expected non-empty task id")
	}
}

func TestCoordinatorExecutor_DoesNotDuplicateSummaryInstruction(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer: "ok",
			Messages: []core.Message{
				{
					Role: "assistant",
					ToolCalls: []core.ToolCall{
						{ID: "call-1", Name: "read_file"},
					},
				},
				{
					Role: "tool",
					ToolResults: []core.ToolResult{
						{CallID: "call-1", Content: "ok"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	prompt := "请完成任务。\n\n## 执行总结\n- 模板"
	if _, err := exec.Execute(context.Background(), "agent-a", prompt, map[string]string{}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if strings.Count(runner.lastPrompt, "## 执行总结") != 1 {
		t.Fatalf("expected one summary instruction block, got prompt: %q", runner.lastPrompt)
	}
}

func TestCoordinatorExecutor_PropagatesChannelContextAndPinnedSelection(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer:   "ok",
			Messages: []core.Message{assistantMessageWithToolCalls("shell_exec")},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)
	exec.SetSelectionResolver(func(_ context.Context, channel, chatID, userID string) (subscription.ResolvedSelection, bool) {
		if channel != "lark" || chatID != "oc_chat" || userID != "ou_user" {
			return subscription.ResolvedSelection{}, false
		}
		return subscription.ResolvedSelection{
			Provider: "codex",
			Model:    "gpt-5.3-codex",
			Pinned:   true,
		}, true
	})

	_, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{
		"channel": "lark",
		"chat_id": "oc_chat",
		"user_id": "ou_user",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if got := appcontext.ChannelFromContext(runner.lastCtx); got != "lark" {
		t.Fatalf("expected channel=lark, got %q", got)
	}
	if got := appcontext.ChatIDFromContext(runner.lastCtx); got != "oc_chat" {
		t.Fatalf("expected chat_id=oc_chat, got %q", got)
	}
	if got := id.UserIDFromContext(runner.lastCtx); got != "ou_user" {
		t.Fatalf("expected user_id=ou_user, got %q", got)
	}
	selection, ok := appcontext.GetLLMSelection(runner.lastCtx)
	if !ok {
		t.Fatal("expected pinned llm selection in context")
	}
	if selection.Provider != "codex" || selection.Model != "gpt-5.3-codex" || !selection.Pinned {
		t.Fatalf("unexpected selection: %#v", selection)
	}
}

func TestCoordinatorExecutor_FailsWithoutRealToolAction(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer:   "## 执行总结\n- 仅规划",
			Messages: []core.Message{assistantMessageWithToolCalls("plan")},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	_, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err == nil {
		t.Fatal("expected failure when no real tool action happened")
	}
	if !strings.Contains(err.Error(), "real tool action") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCoordinatorExecutor_FailsWhenRealToolResultIsError(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer: "## 执行总结\n- shell 执行失败",
			Messages: []core.Message{
				{
					Role: "assistant",
					ToolCalls: []core.ToolCall{
						{ID: "call-1", Name: "shell_exec"},
					},
				},
				{
					Role: "tool",
					ToolResults: []core.ToolResult{
						{CallID: "call-1", Error: fmt.Errorf("exit status 1")},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	_, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err == nil {
		t.Fatal("expected failure when real tool result failed")
	}
	if !strings.Contains(err.Error(), "real tool action") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCoordinatorExecutor_FailsWhenResultAwaitsUserConfirmation(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer: "## 执行总结\n- 我这边先不继续硬写 `~/.alex/kernel/default/...` 了。\n- 我的理解是你要我下一轮改成 `./kernel_sync/...`，然后由 kernel 同步——对吗？\n- 可选：A) 按 `./kernel_sync/knowledge|goal|drafts` 执行 B) 你指定路径",
			Messages: []core.Message{
				{
					Role: "assistant",
					ToolCalls: []core.ToolCall{
						{ID: "call-1", Name: "read_file"},
					},
				},
				{
					Role: "tool",
					ToolResults: []core.ToolResult{
						{CallID: "call-1", Content: "ok"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	_, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err == nil {
		t.Fatal("expected failure when result still awaits user confirmation")
	}
	if !strings.Contains(err.Error(), "awaiting user confirmation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCoordinatorExecutor_FailsWhenStopReasonAwaitUserInput(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer:     "## 执行总结\n- 等待用户确认",
			StopReason: "await_user_input",
			Messages: []core.Message{
				{
					Role: "assistant",
					ToolCalls: []core.ToolCall{
						{ID: "call-1", Name: "shell_exec"},
					},
				},
				{
					Role: "tool",
					ToolResults: []core.ToolResult{
						{CallID: "call-1", Content: "ok"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	_, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err == nil {
		t.Fatal("expected failure when stop reason is await_user_input")
	}
	if !strings.Contains(err.Error(), "awaiting user confirmation") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCoordinatorExecutor_RetriesAutonomouslyAfterConfirmationLoop(t *testing.T) {
	runner := &stubTaskRunner{
		results: []*agent.TaskResult{
			{
				Answer: "## 执行总结\n- 我的理解是你要我改成 `./kernel_sync/...`——对吗？",
				Messages: []core.Message{
					{
						Role: "assistant",
						ToolCalls: []core.ToolCall{
							{ID: "call-1", Name: "read_file"},
						},
					},
					{
						Role: "tool",
						ToolResults: []core.ToolResult{
							{CallID: "call-1", Content: "ok"},
						},
					},
				},
			},
			{
				Answer: "## 执行总结\n- 已写入 ./kernel_sync/knowledge/topic.md\n- 已更新 ./kernel_sync/goal/GOAL.md",
				Messages: []core.Message{
					{
						Role: "assistant",
						ToolCalls: []core.ToolCall{
							{ID: "call-2", Name: "write_file"},
						},
					},
					{
						Role: "tool",
						ToolResults: []core.ToolResult{
							{CallID: "call-2", Content: "ok"},
						},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	res, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err != nil {
		t.Fatalf("expected autonomous retry success, got error: %v", err)
	}
	if strings.TrimSpace(res.Summary) == "" {
		t.Fatalf("expected non-empty summary after retry")
	}
	if len(runner.prompts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(runner.prompts))
	}
	if !strings.Contains(runner.prompts[1], "Kernel retry requirement") {
		t.Fatalf("expected retry prompt guidance, got: %q", runner.prompts[1])
	}
	if res.Attempts != 2 {
		t.Fatalf("expected attempts=2 after retry, got %d", res.Attempts)
	}
	if res.RecoveredFrom != kernelAutonomyAwaiting {
		t.Fatalf("expected recovered_from=%s, got %q", kernelAutonomyAwaiting, res.RecoveredFrom)
	}
	if res.Autonomy != kernelAutonomyActionable {
		t.Fatalf("expected autonomy=%s, got %q", kernelAutonomyActionable, res.Autonomy)
	}
}

func TestCoordinatorExecutor_RetriesWhenFirstAttemptHasNoRealToolAction(t *testing.T) {
	runner := &stubTaskRunner{
		results: []*agent.TaskResult{
			{
				Answer: "## 执行总结\n- 已规划下一步",
				Messages: []core.Message{
					{
						Role: "assistant",
						ToolCalls: []core.ToolCall{
							{ID: "call-1", Name: "plan"},
						},
					},
				},
			},
			{
				Answer: "## 执行总结\n- 已执行 read_file 并更新草稿",
				Messages: []core.Message{
					{
						Role: "assistant",
						ToolCalls: []core.ToolCall{
							{ID: "call-2", Name: "read_file"},
						},
					},
					{
						Role: "tool",
						ToolResults: []core.ToolResult{
							{CallID: "call-2", Content: "ok"},
						},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	res, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err != nil {
		t.Fatalf("expected retry success after no-real-tool first attempt, got: %v", err)
	}
	if strings.TrimSpace(res.Summary) == "" {
		t.Fatalf("expected non-empty summary after retry")
	}
	if len(runner.prompts) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(runner.prompts))
	}
	if res.Attempts != 2 {
		t.Fatalf("expected attempts=2 after retry, got %d", res.Attempts)
	}
	if res.RecoveredFrom != kernelAutonomyNoTool {
		t.Fatalf("expected recovered_from=%s, got %q", kernelAutonomyNoTool, res.RecoveredFrom)
	}
}

func TestCoordinatorExecutor_DoesNotRetryWhenFirstAttemptIsValid(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer: "## 执行总结\n- 已执行 read_file 并完成分析",
			Messages: []core.Message{
				{
					Role: "assistant",
					ToolCalls: []core.ToolCall{
						{ID: "call-1", Name: "read_file"},
					},
				},
				{
					Role: "tool",
					ToolResults: []core.ToolResult{
						{CallID: "call-1", Content: "ok"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	res, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err != nil {
		t.Fatalf("expected success without retry, got: %v", err)
	}
	if len(runner.prompts) != 1 {
		t.Fatalf("expected single attempt for valid result, got %d", len(runner.prompts))
	}
	if res.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", res.Attempts)
	}
	if res.RecoveredFrom != "" {
		t.Fatalf("expected empty recovered_from for first-attempt success, got %q", res.RecoveredFrom)
	}
}

func TestCoordinatorExecutor_AllowsSuccessfulRealToolResult(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer: "## 执行总结\n- shell 成功",
			Messages: []core.Message{
				{
					Role: "assistant",
					ToolCalls: []core.ToolCall{
						{ID: "call-1", Name: "shell_exec"},
					},
				},
				{
					Role: "tool",
					ToolResults: []core.ToolResult{
						{CallID: "call-1", Content: "ok"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	res, err := exec.Execute(context.Background(), "agent-a", "执行真实任务", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(res.Summary) == "" {
		t.Fatalf("expected non-empty summary: %#v", res)
	}
}

func TestCompactSummary_TruncatesByRunes(t *testing.T) {
	got := compactSummary("这是一个用于验证中文截断不乱码的执行总结内容", 12)
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected truncated summary, got %q", got)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8 summary, got %q", got)
	}
}

func TestCoordinatorExecutor_SetsAutoApproveInContext(t *testing.T) {
	runner := &stubTaskRunner{
		result: &agent.TaskResult{
			Answer:   "## 执行总结\n- done",
			Messages: []core.Message{assistantMessageWithToolCalls("shell_exec")},
		},
	}
	exec := NewCoordinatorExecutor(runner, 0)

	_, err := exec.Execute(context.Background(), "agent-a", "test auto-approve", map[string]string{})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !toolshared.GetAutoApproveFromContext(runner.lastCtx) {
		t.Fatal("expected auto-approve=true in kernel dispatch context")
	}
}

func assistantMessageWithToolCalls(toolNames ...string) core.Message {
	msg := core.Message{Role: "assistant"}
	msg.ToolCalls = make([]core.ToolCall, 0, len(toolNames))
	for i, name := range toolNames {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		msg.ToolCalls = append(msg.ToolCalls, core.ToolCall{
			ID:   "call-" + strconv.Itoa(i+1),
			Name: trimmed,
		})
	}
	return msg
}
