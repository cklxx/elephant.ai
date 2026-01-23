package builtin

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
)

func TestACPExecutorHandlerEnsuresManifest(t *testing.T) {
	call := ports.ToolCall{ID: "call-1", SessionID: "sess", TaskID: "task"}
	handler := newACPExecutorHandler(context.Background(), call, 0, true, false, nil, nil)

	if err := handler.finish(); err != nil {
		t.Fatalf("expected fallback manifest, got %v", err)
	}
	if handler.manifestPayload() == nil {
		t.Fatalf("expected fallback manifest payload")
	}
	if !handler.isManifestMissing() {
		t.Fatalf("expected manifest missing flag to be true")
	}

	handler = newACPExecutorHandler(context.Background(), call, 0, true, false, nil, nil)
	handler.handleArtifactManifest("artifact_manifest", `{"items":[{"type":"patch"}]}`, nil)
	if err := handler.finish(); err != nil {
		t.Fatalf("expected manifest requirement to pass, got %v", err)
	}
	if handler.isManifestMissing() {
		t.Fatalf("expected manifest missing flag to be false when executor emitted manifest")
	}
}
