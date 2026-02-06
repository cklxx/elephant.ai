package react

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	tools "alex/internal/domain/agent/ports/tools"
)

type stubToolLimiter struct {
	limit int
}

func (s stubToolLimiter) Limit() int {
	return s.limit
}

func TestToolCallBatchRespectsConcurrencyLimit(t *testing.T) {
	var inFlight int32
	var maxSeen int32

	executor := &mocks.MockToolExecutor{
		ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
			current := atomic.AddInt32(&inFlight, 1)
			for {
				max := atomic.LoadInt32(&maxSeen)
				if current <= max {
					break
				}
				if atomic.CompareAndSwapInt32(&maxSeen, max, current) {
					break
				}
			}
			time.Sleep(40 * time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
			return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
		},
	}

	registry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return executor, nil
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		Logger: agent.NoopLogger{},
		Clock:  agent.SystemClock{},
	})

	state := &TaskState{SessionID: "sess", RunID: "task"}
	calls := []ToolCall{
		{ID: "call-1", Name: "t1"},
		{ID: "call-2", Name: "t2"},
		{ID: "call-3", Name: "t3"},
		{ID: "call-4", Name: "t4"},
	}

	batch := newToolCallBatch(engine, context.Background(), state, 1, calls, registry, stubToolLimiter{limit: 2}, nil)
	batch.execute()

	if maxSeen != 2 {
		t.Fatalf("expected max concurrency 2, got %d", maxSeen)
	}
}
