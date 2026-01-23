package builtin

import (
	"context"
	"testing"

	"alex/internal/agent/ports"
)

func TestArtifactManifestRequiresItems(t *testing.T) {
	tool := NewArtifactManifest()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Name:      "artifact_manifest",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected tool error when items missing, got %#v", result)
	}
}

func TestArtifactManifestEmitsAttachment(t *testing.T) {
	tool := NewArtifactManifest()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:   "call-2",
		Name: "artifact_manifest",
		Arguments: map[string]any{
			"items": []any{
				map[string]any{"type": "patch", "path": "changes.diff"},
			},
			"summary": "Generated artifacts.",
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected successful result, got %#v", result)
	}
	if len(result.Attachments) != 1 {
		t.Fatalf("expected one attachment, got %d", len(result.Attachments))
	}
	var att ports.Attachment
	for _, value := range result.Attachments {
		att = value
	}
	if att.Format != "manifest" || att.Kind != "artifact" {
		t.Fatalf("unexpected attachment metadata: %+v", att)
	}
	if result.Metadata == nil {
		t.Fatalf("expected metadata to include manifest")
	}
	if _, ok := result.Metadata["artifact_manifest"]; !ok {
		t.Fatalf("expected artifact_manifest metadata, got %#v", result.Metadata)
	}
}
