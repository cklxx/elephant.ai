package builtin

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
)

func TestACPExecutorHandlerRequiresManifest(t *testing.T) {
	call := ports.ToolCall{ID: "call-1", SessionID: "sess", TaskID: "task"}
	handler := newACPExecutorHandler(context.Background(), call, 0, true, false, nil, nil)

	if err := handler.finish(); err == nil {
		t.Fatalf("expected error when manifest is required but missing")
	}

	handler.handleArtifactManifest("artifact_manifest", `{"items":[{"type":"patch"}]}`, nil)
	if err := handler.finish(); err != nil {
		t.Fatalf("expected manifest requirement to pass, got %v", err)
	}
}
