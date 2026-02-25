package lark

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/domain/agent/types"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

type slowSummaryStubLLMClient struct {
	mu   sync.Mutex
	resp string
	reqs []ports.CompletionRequest
}

func (c *slowSummaryStubLLMClient) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	return &ports.CompletionResponse{Content: c.resp}, nil
}

func (c *slowSummaryStubLLMClient) Model() string { return "stub-slow-summary" }

func (c *slowSummaryStubLLMClient) RequestCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.reqs)
}

type slowSummaryStubFactory struct {
	client portsllm.LLMClient
}

func (f *slowSummaryStubFactory) GetClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *slowSummaryStubFactory) GetIsolatedClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *slowSummaryStubFactory) DisableRetry() {}

func TestSlowProgressSummaryListener_SendsSummaryAfterDelay(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		messenger: recorder,
		logger:    logging.NewComponentLogger("test"),
	}
	gw.activeSlots.Store("oc_chat", &sessionSlot{phase: slotRunning})

	ln := newSlowProgressSummaryListener(
		context.Background(),
		agent.NoopEventListener{},
		gw,
		"oc_chat",
		"om_parent",
		20*time.Millisecond,
	)
	defer ln.Close()

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventToolStarted,
		NodeKind:  "tool",
		NodeID:    "call-1",
		Payload: map[string]any{
			"tool_name": "read_file",
		},
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		replies := recorder.CallsByMethod("ReplyMessage")
		if len(replies) > 0 {
			text := extractTextContent(replies[len(replies)-1].Content, nil)
			if text == "" {
				t.Fatalf("expected non-empty summary text")
			}
			if got := replies[len(replies)-1].ReplyTo; got != "om_parent" {
				t.Fatalf("expected reply target om_parent, got %q", got)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for slow progress summary message")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSlowProgressSummaryListener_NoSummaryWhenTerminalBeforeDelay(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		messenger: recorder,
		logger:    logging.NewComponentLogger("test"),
	}
	gw.activeSlots.Store("oc_chat", &sessionSlot{phase: slotRunning})

	ln := newSlowProgressSummaryListener(
		context.Background(),
		agent.NoopEventListener{},
		gw,
		"oc_chat",
		"om_parent",
		40*time.Millisecond,
	)
	defer ln.Close()

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventToolStarted,
		NodeKind:  "tool",
		NodeID:    "call-1",
		Payload: map[string]any{
			"tool_name": "read_file",
		},
	})
	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventResultFinal,
		NodeKind:  "result",
		NodeID:    "final",
		Payload: map[string]any{
			"final_answer": "done",
		},
	})

	time.Sleep(120 * time.Millisecond)
	if replies := recorder.CallsByMethod("ReplyMessage"); len(replies) != 0 {
		t.Fatalf("expected no summary after terminal event, got %d replies", len(replies))
	}
}

func TestSlowProgressSummaryListener_UsesLLMSummaryWhenAvailable(t *testing.T) {
	recorder := NewRecordingMessenger()
	client := &slowSummaryStubLLMClient{resp: "- 已完成上下文准备\n- 正在执行工具 read_file"}
	gw := &Gateway{
		messenger:  recorder,
		logger:     logging.NewComponentLogger("test"),
		llmFactory: &slowSummaryStubFactory{client: client},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}
	gw.activeSlots.Store("oc_chat", &sessionSlot{phase: slotRunning})

	ln := newSlowProgressSummaryListener(
		context.Background(),
		agent.NoopEventListener{},
		gw,
		"oc_chat",
		"om_parent",
		20*time.Millisecond,
	)
	defer ln.Close()

	ln.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventNodeStarted,
		NodeKind:  "step",
		NodeID:    "step-1",
		Payload: map[string]any{
			"step_description": "准备上下文",
		},
	})

	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		replies := recorder.CallsByMethod("ReplyMessage")
		if len(replies) > 0 {
			text := extractTextContent(replies[len(replies)-1].Content, nil)
			if !containsAll(text, "任务已运行", "最近进展", "已完成上下文准备") {
				t.Fatalf("expected LLM summary text, got %q", text)
			}
			if client.RequestCount() == 0 {
				t.Fatalf("expected LLM request to be issued")
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for LLM slow summary message")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func containsAll(text string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(text, sub) {
			return false
		}
	}
	return true
}
