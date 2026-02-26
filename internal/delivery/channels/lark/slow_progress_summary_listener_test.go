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
	client := &slowSummaryStubLLMClient{resp: "这边 30s 进展：前两轮已经跑完了，现在在第 3 轮思考中，整体推进正常，我会在出最终结果后第一时间同步你。"}
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
			if !containsAll(text, "这边 30s 进展", "前两轮已经跑完了") {
				t.Fatalf("expected LLM summary text, got %q", text)
			}
			if strings.Contains(text, "最近工具调用（人话）") {
				t.Fatalf("expected pure LLM summary without tool appendix, got %q", text)
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

func TestBuildSlowSummaryIntervals_DefaultCadence(t *testing.T) {
	got := buildSlowSummaryIntervals(30 * time.Second)
	if len(got) != 3 {
		t.Fatalf("expected 3 cadence intervals, got %d", len(got))
	}
	want := []time.Duration{30 * time.Second, 60 * time.Second, 180 * time.Second}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("cadence[%d] = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestSlowProgressSummaryListener_RepeatsSummaryByCadence(t *testing.T) {
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
		20*time.Millisecond, // cadence: 20ms, 40ms, 120ms...
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
		if len(replies) >= 2 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected repeated slow summaries, got %d reply/replies", len(replies))
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestSlowProgressSummaryListener_FallbackIncludesHumanToolSummary(t *testing.T) {
	ln := newSlowProgressSummaryListener(
		context.Background(),
		agent.NoopEventListener{},
		nil,
		"oc_chat",
		"om_parent",
		20*time.Millisecond,
	)
	defer ln.Close()

	text := ln.buildFallbackSummary([]slowProgressSignal{
		{at: time.Now(), text: "开始工具：read_file"},
		{at: time.Now(), text: "完成工具：web_search"},
	}, 35*time.Second)

	if !containsAll(text, "最近工具调用（人话）", "read_file", "web_search") {
		t.Fatalf("expected humanized tool summary in fallback text, got %q", text)
	}
}

func TestSignalFromEnvelope_HumanizesReactNodeID(t *testing.T) {
	signal, ok := signalFromEnvelope(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventNodeStarted,
		NodeKind:  "step",
		NodeID:    "react:iter:3:think",
	})
	if !ok {
		t.Fatal("expected react node id to produce signal")
	}
	if signal.text != "开始步骤：第 3 轮思考" {
		t.Fatalf("unexpected signal text: %q", signal.text)
	}
}

func TestSignalFromEnvelope_DropsInternalSummaryContent(t *testing.T) {
	_, ok := signalFromEnvelope(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "run", "", time.Now()),
		Event:     types.EventNodeOutputSummary,
		NodeKind:  "step",
		NodeID:    "react:iter:2:plan",
		Payload: map[string]any{
			"content": "已完成 react:iter:2:tool:call_qPikHBe7pX5L3EtTMskrisMj",
		},
	})
	if ok {
		t.Fatal("expected internal summary content to be filtered out")
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
