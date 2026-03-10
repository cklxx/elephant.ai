package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"slices"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

// e2eExecutor returns a configurable result and records calls with proper synchronization.
type e2eExecutor struct {
	mu           sync.Mutex
	result       *agent.TaskResult
	err          error
	panicMsg     string
	called       bool
	capturedTask string
}

func (e *e2eExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *e2eExecutor) ExecuteTask(_ context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	e.mu.Lock()
	e.called = true
	e.capturedTask = task
	e.mu.Unlock()
	if e.panicMsg != "" {
		panic(e.panicMsg)
	}
	return e.result, e.err
}

func (e *e2eExecutor) wasCalled() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.called
}

func (e *e2eExecutor) getCapturedTask() string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.capturedTask
}

// TestE2E_NormalMessageFlow verifies the happy path: message → executor → reply.
func TestE2E_NormalMessageFlow(t *testing.T) {
	rec := NewRecordingMessenger()
	exec := &e2eExecutor{
		result: &agent.TaskResult{Answer: "Hello from AI"},
	}
	gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
		SessionPrefix: "e2e",
		AllowDirect:   true,
	})

	err := gw.InjectMessage(context.Background(), "oc_e2e_chat", "p2p", "ou_sender", "om_e2e_msg_1", "Hello")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	if !exec.wasCalled() {
		t.Fatal("expected executor to be called")
	}
	if task := exec.getCapturedTask(); !strings.Contains(task, "Hello") {
		t.Fatalf("expected captured task to contain 'Hello', got %q", task)
	}

	// The gateway uses SendMessage when the messageID is synthetic (inject IDs
	// are not valid Lark open_message_ids, so replyTarget falls back to "").
	// Check both ReplyMessage and SendMessage.
	replies := rec.CallsByMethod(MethodReplyMessage)
	sends := rec.CallsByMethod(MethodSendMessage)
	allOutbound := slices.Concat(replies, sends)
	if len(allOutbound) == 0 {
		t.Fatal("expected at least one outbound message (ReplyMessage or SendMessage)")
	}

	found := false
	for _, call := range allOutbound {
		if strings.Contains(call.Content, "Hello from AI") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reply containing 'Hello from AI', got calls: %+v", allOutbound)
	}
}

// TestE2E_AttentionGate_LowUrgencyAutoAck verifies that a non-urgent message
// triggers an auto-ack reply and does NOT invoke the executor.
func TestE2E_AttentionGate_LowUrgencyAutoAck(t *testing.T) {
	rec := NewRecordingMessenger()
	exec := &e2eExecutor{
		result: &agent.TaskResult{Answer: "should not appear"},
	}
	gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
		SessionPrefix: "e2e",
		AllowDirect:   true,
	})

	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"urgent", "P0"},
		AutoAckMessage: "已收到",
	})
	gw.attentionGate = gate

	// "请帮我查看代码" is a routine message — no urgent keywords.
	err := gw.InjectMessage(context.Background(), "oc_e2e_gate", "p2p", "ou_sender", "om_e2e_gate_1", "请帮我查看代码")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	// Give a moment for any synchronous dispatch.
	time.Sleep(100 * time.Millisecond)

	if exec.wasCalled() {
		t.Fatal("executor should NOT be called for low-urgency gated message")
	}

	// Auto-ack may be sent via ReplyMessage or SendMessage depending on messageID type.
	replies := rec.CallsByMethod(MethodReplyMessage)
	sends := rec.CallsByMethod(MethodSendMessage)
	allOutbound := slices.Concat(replies, sends)
	if len(allOutbound) == 0 {
		t.Fatal("expected auto-ack reply")
	}

	found := false
	for _, call := range allOutbound {
		if strings.Contains(call.Content, "已收到") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected auto-ack containing '已收到', got calls: %+v", allOutbound)
	}
}

// TestE2E_AttentionGate_HighUrgencyPassthrough verifies that an urgent message
// bypasses the attention gate and is processed normally by the executor.
func TestE2E_AttentionGate_HighUrgencyPassthrough(t *testing.T) {
	rec := NewRecordingMessenger()
	exec := &e2eExecutor{
		result: &agent.TaskResult{Answer: "处理中"},
	}
	gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
		SessionPrefix: "e2e",
		AllowDirect:   true,
	})

	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"urgent", "P0"},
		AutoAckMessage: "已收到",
	})
	gw.attentionGate = gate

	// "P0 服务器宕机" contains urgent keyword "P0".
	err := gw.InjectMessage(context.Background(), "oc_e2e_urgent", "p2p", "ou_sender", "om_e2e_urgent_1", "P0 服务器宕机")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	if !exec.wasCalled() {
		t.Fatal("executor SHOULD be called for high-urgency message")
	}
	if task := exec.getCapturedTask(); !strings.Contains(task, "P0") {
		t.Fatalf("expected captured task to contain 'P0', got %q", task)
	}

	// Should get a normal reply (not auto-ack).
	replies := rec.CallsByMethod(MethodReplyMessage)
	sends := rec.CallsByMethod(MethodSendMessage)
	allOutbound := slices.Concat(replies, sends)
	if len(allOutbound) == 0 {
		t.Fatal("expected at least one reply for urgent message")
	}

	found := false
	for _, call := range allOutbound {
		if strings.Contains(call.Content, "处理中") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reply containing '处理中', got calls: %+v", allOutbound)
	}
}

// TestE2E_FocusTime_Suppression verifies that the attention gate's ShouldDispatch
// method correctly suppresses messages when the focus time checker indicates suppression.
// Note: The gateway handler currently wires ClassifyUrgency+RecordDispatch directly
// (not ShouldDispatch), so focus time is tested at the gate API level to ensure the
// component contract is correct.
func TestE2E_FocusTime_Suppression(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0"},
	})
	gate.SetFocusTimeChecker(&mockFocusChecker{suppressed: map[string]bool{"ou_focus_user": true}})

	// Non-urgent message during focus time → suppressed.
	urgency, shouldDispatch := gate.ShouldDispatch("请帮我查看代码", "chat-1", "ou_focus_user", time.Now())
	if shouldDispatch {
		t.Fatal("expected message to be suppressed during focus time")
	}
	if urgency != UrgencyLow {
		t.Fatalf("expected UrgencyLow, got %d", urgency)
	}

	// Urgent message during focus time → still passes through.
	urgency, shouldDispatch = gate.ShouldDispatch("P0 incident", "chat-1", "ou_focus_user", time.Now())
	if !shouldDispatch {
		t.Fatal("expected urgent message to bypass focus time suppression")
	}
	if urgency != UrgencyHigh {
		t.Fatalf("expected UrgencyHigh, got %d", urgency)
	}

	// Non-focus user → not suppressed.
	_, shouldDispatch = gate.ShouldDispatch("routine msg", "chat-1", "ou_other_user", time.Now())
	if !shouldDispatch {
		t.Fatal("expected message from non-focus user to pass through")
	}
}

// TestE2E_ExecutorError_Degradation verifies that when the executor returns
// an error, the gateway sends a user-friendly error reply.
func TestE2E_ExecutorError_Degradation(t *testing.T) {
	rec := NewRecordingMessenger()
	exec := &e2eExecutor{
		result: nil,
		err:    fmt.Errorf("LLM provider timeout"),
	}
	gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
		SessionPrefix: "e2e",
		AllowDirect:   true,
	})

	err := gw.InjectMessage(context.Background(), "oc_e2e_err", "p2p", "ou_sender", "om_e2e_err_1", "please do something")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	// Wait for async reply dispatch.
	time.Sleep(200 * time.Millisecond)

	replies := rec.CallsByMethod(MethodReplyMessage)
	sends := rec.CallsByMethod(MethodSendMessage)
	allOutbound := slices.Concat(replies, sends)
	if len(allOutbound) == 0 {
		t.Fatal("expected an error reply to be sent")
	}

	// The reply should contain some indication of failure (localized or otherwise).
	// The gateway wraps errors with "执行失败" or uses BuildReplyCore.
	found := false
	for _, call := range allOutbound {
		content := call.Content
		if strings.Contains(content, "失败") || strings.Contains(content, "error") ||
			strings.Contains(content, "timeout") || strings.Contains(content, "重试") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected error-related reply, got calls: %+v", allOutbound)
	}
}

// TestE2E_MultipleMessagesSequential verifies that two sequential messages
// to different chats are processed independently without interference.
func TestE2E_MultipleMessagesSequential(t *testing.T) {
	rec := NewRecordingMessenger()
	var mu sync.Mutex
	var tasks []string
	exec := &orderTrackingExecutor{
		ensureFn: func(_ context.Context, sid string) (*storage.Session, error) {
			return &storage.Session{ID: sid, Metadata: map[string]string{}}, nil
		},
		executeFn: func(_ context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
			mu.Lock()
			tasks = append(tasks, task)
			mu.Unlock()
			return &agent.TaskResult{Answer: "reply-" + task}, nil
		},
	}
	gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
		SessionPrefix: "e2e",
		AllowDirect:   true,
	})

	// First message.
	err := gw.InjectMessage(context.Background(), "oc_chat_a", "p2p", "ou_user_a", "om_seq_1", "first")
	if err != nil {
		t.Fatalf("first InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	// Second message to a different chat.
	err = gw.InjectMessage(context.Background(), "oc_chat_b", "p2p", "ou_user_b", "om_seq_2", "second")
	if err != nil {
		t.Fatalf("second InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	mu.Lock()
	defer mu.Unlock()
	if len(tasks) != 2 {
		t.Fatalf("expected 2 tasks processed, got %d: %v", len(tasks), tasks)
	}

	// Both replies should be present.
	replies := rec.CallsByMethod(MethodReplyMessage)
	sends := rec.CallsByMethod(MethodSendMessage)
	allOutbound := slices.Concat(replies, sends)
	if len(allOutbound) < 2 {
		t.Fatalf("expected at least 2 outbound messages, got %d", len(allOutbound))
	}
}
