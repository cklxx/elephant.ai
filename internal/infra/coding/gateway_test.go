package coding

import (
	"context"
	"testing"
	"time"
)

type stubAdapter struct {
	name         string
	submitCalled bool
	streamCalled bool
}

func (s *stubAdapter) Name() string { return s.name }

func (s *stubAdapter) Submit(_ context.Context, req TaskRequest) (*TaskResult, error) {
	s.submitCalled = true
	return &TaskResult{TaskID: req.TaskID, Answer: "ok"}, nil
}

func (s *stubAdapter) Stream(_ context.Context, req TaskRequest, _ ProgressCallback) (*TaskResult, error) {
	s.streamCalled = true
	return &TaskResult{TaskID: req.TaskID, Answer: "stream"}, nil
}

func (s *stubAdapter) Cancel(_ context.Context, _ string) error {
	return ErrNotSupported
}

func (s *stubAdapter) Status(_ context.Context, _ string) (TaskStatus, error) {
	return TaskStatus{State: TaskStateRunning, UpdatedAt: time.Now()}, nil
}

func TestGatewaySelectsDefaultAdapter(t *testing.T) {
	registry := NewAdapterRegistry()
	adapter := &stubAdapter{name: "codex"}
	if err := registry.Register(adapter); err != nil {
		t.Fatalf("register: %v", err)
	}
	gw := NewGateway(registry, "codex")

	result, err := gw.Submit(context.Background(), TaskRequest{TaskID: "t1", Prompt: "hi"})
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if result == nil || result.Answer != "ok" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !adapter.submitCalled {
		t.Fatalf("expected submit to be called")
	}
}

func TestGatewayRequiresAgentTypeWhenMultiple(t *testing.T) {
	registry := NewAdapterRegistry()
	if err := registry.Register(&stubAdapter{name: "codex"}); err != nil {
		t.Fatalf("register codex: %v", err)
	}
	if err := registry.Register(&stubAdapter{name: "claude_code"}); err != nil {
		t.Fatalf("register claude: %v", err)
	}
	gw := NewGateway(registry, "")

	_, err := gw.Submit(context.Background(), TaskRequest{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected error when multiple adapters and no agent_type")
	}
}

func TestGatewayStreamUsesAdapterStream(t *testing.T) {
	registry := NewAdapterRegistry()
	adapter := &stubAdapter{name: "codex"}
	if err := registry.Register(adapter); err != nil {
		t.Fatalf("register: %v", err)
	}
	gw := NewGateway(registry, "codex")

	_, err := gw.Stream(context.Background(), TaskRequest{TaskID: "t1"}, func(TaskProgress) {})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if !adapter.streamCalled {
		t.Fatalf("expected stream to be called")
	}
}

func TestAdapterRegistryErrors(t *testing.T) {
	registry := NewAdapterRegistry()
	if err := registry.Register(nil); err == nil {
		t.Fatal("expected error for nil adapter")
	}
	adapter := &stubAdapter{name: "codex"}
	if err := registry.Register(adapter); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := registry.Register(adapter); err == nil {
		t.Fatal("expected error for duplicate adapter")
	}
	if _, err := registry.Get("missing"); err == nil {
		t.Fatal("expected error for missing adapter")
	}
	if _, err := registry.Get("codex"); err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}
}

func TestGatewayNoAdapters(t *testing.T) {
	gw := NewGateway(NewAdapterRegistry(), "")
	_, err := gw.Submit(context.Background(), TaskRequest{TaskID: "t1"})
	if err == nil {
		t.Fatal("expected error when no adapters")
	}
}
