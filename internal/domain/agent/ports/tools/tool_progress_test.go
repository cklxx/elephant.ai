package tools

import (
	"context"
	"testing"
)

func TestWithToolProgressEmitterHandlesNilInputs(t *testing.T) {
	emitter := ToolProgressEmitter(func(string, bool) {})
	if got := WithToolProgressEmitter(nil, emitter); got != nil {
		t.Fatalf("expected nil context to stay nil, got %#v", got)
	}

	ctx := context.Background()
	if got := WithToolProgressEmitter(ctx, nil); got != ctx {
		t.Fatalf("expected nil emitter to leave context unchanged")
	}

	if got := GetToolProgressEmitter(nil); got != nil {
		t.Fatalf("expected nil context to return no emitter, got %#v", got)
	}

	EmitToolProgress(context.Background(), "ignored", true)
}

func TestToolProgressEmitterRoundTripAndEmit(t *testing.T) {
	var chunks []string
	var complete []bool
	emitter := ToolProgressEmitter(func(chunk string, isComplete bool) {
		chunks = append(chunks, chunk)
		complete = append(complete, isComplete)
	})

	ctx := WithToolProgressEmitter(context.Background(), emitter)
	if got := GetToolProgressEmitter(ctx); got == nil {
		t.Fatal("expected emitter to round-trip through context")
	}

	EmitToolProgress(ctx, "phase 1", false)
	EmitToolProgress(ctx, "done", true)

	if len(chunks) != 2 {
		t.Fatalf("expected 2 emitted chunks, got %d", len(chunks))
	}
	if chunks[0] != "phase 1" || complete[0] {
		t.Fatalf("unexpected first emission: chunk=%q complete=%v", chunks[0], complete[0])
	}
	if chunks[1] != "done" || !complete[1] {
		t.Fatalf("unexpected final emission: chunk=%q complete=%v", chunks[1], complete[1])
	}
}
