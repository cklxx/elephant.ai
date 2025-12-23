package builtin

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestArtifactsWriteAddsOrUpdatesAttachments(t *testing.T) {
	tool := NewArtifactsWrite()

	call := ports.ToolCall{ID: "call-1", Name: "artifacts_write", Arguments: map[string]any{
		"name":        "note.md",
		"content":     "# Title\nBody",
		"description": "meeting notes",
	}}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}

	att, ok := result.Attachments["note.md"]
	if !ok {
		t.Fatalf("expected attachment in result: %+v", result.Attachments)
	}
	if att.MediaType != "text/markdown" {
		t.Fatalf("unexpected media type: %s", att.MediaType)
	}
	if att.Source != "artifacts_write" {
		t.Fatalf("expected source to default to tool name, got %q", att.Source)
	}
	if att.Format != "markdown" {
		t.Fatalf("expected markdown format normalization, got %q", att.Format)
	}
	if att.PreviewProfile != "document.markdown" {
		t.Fatalf("expected markdown preview profile, got %q", att.PreviewProfile)
	}

	mutationsRaw, ok := result.Metadata["attachment_mutations"]
	if !ok || mutationsRaw == nil {
		t.Fatalf("expected attachment mutations metadata, got: %+v", result.Metadata)
	}
	mutations, ok := mutationsRaw.(map[string]any)
	if !ok {
		t.Fatalf("unexpected mutation payload type: %T", mutationsRaw)
	}
	addRaw, ok := mutations["add"]
	if !ok {
		t.Fatalf("expected add mutation for note.md: %+v", mutations)
	}
	add, ok := addRaw.(map[string]ports.Attachment)
	if !ok {
		t.Fatalf("unexpected add mutation type: %T", addRaw)
	}
	if _, ok := add["note.md"]; !ok {
		t.Fatalf("expected add mutation for note.md: %+v", add)
	}

	// Execute again with existing attachment to ensure we switch to update
	ctx := ports.WithAttachmentContext(context.Background(), result.Attachments, nil)
	updated, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error on update: %v", err)
	}
	updatesRaw := updated.Metadata["attachment_mutations"]
	updates, ok := updatesRaw.(map[string]any)
	if !ok {
		t.Fatalf("unexpected mutations payload on update: %T", updatesRaw)
	}
	updateRaw, ok := updates["update"]
	if !ok {
		t.Fatalf("expected update mutation when attachment already exists: %+v", updates)
	}
	if _, ok := updateRaw.(map[string]ports.Attachment)["note.md"]; !ok {
		t.Fatalf("expected update mutation for note.md: %+v", updateRaw)
	}
}

func TestArtifactsListReturnsSnapshot(t *testing.T) {
	notePayload := base64.StdEncoding.EncodeToString([]byte("content"))
	attachments := map[string]ports.Attachment{
		"note.md": {
			Name:        "note.md",
			MediaType:   "text/markdown",
			Data:        notePayload,
			URI:         "data:text/markdown;base64," + notePayload,
			Description: "notes",
		},
		"image.png": {Name: "image.png", MediaType: "image/png"},
	}
	ctx := ports.WithAttachmentContext(context.Background(), attachments, nil)

	tool := NewArtifactsList()
	call := ports.ToolCall{ID: "list-1", Name: "artifacts_list"}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "note.md") || !strings.Contains(result.Content, "image.png") {
		t.Fatalf("expected listing to include attachment names, got: %s", result.Content)
	}

	// Request a specific attachment payload
	call.Arguments = map[string]any{"name": "note.md"}
	targeted, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := targeted.Attachments["note.md"]; !ok {
		t.Fatalf("expected targeted attachment to be returned: %+v", targeted.Attachments)
	}

	call.Arguments = map[string]any{"name": "[note.md]"}
	targeted, err = tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := targeted.Attachments["note.md"]; !ok {
		t.Fatalf("expected placeholder name to resolve: %+v", targeted.Attachments)
	}

	call.Arguments = map[string]any{"name": attachments["note.md"].URI}
	targeted, err = tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := targeted.Attachments["note.md"]; !ok {
		t.Fatalf("expected data URI to resolve: %+v", targeted.Attachments)
	}
}

func TestArtifactsDeleteBuildsRemovalMutation(t *testing.T) {
	tool := NewArtifactsDelete()
	call := ports.ToolCall{ID: "del-1", Name: "artifacts_delete", Arguments: map[string]any{
		"names": []string{"[old.md]", "draft.txt"},
	}}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mutations := result.Metadata["attachment_mutations"].(map[string]any)
	removed, ok := mutations["remove"].([]string)
	if !ok || len(removed) != 2 {
		t.Fatalf("expected removal entries for two attachments, got: %+v", mutations)
	}
	if removed[0] != "old.md" {
		t.Fatalf("expected placeholder to unwrap to old.md, got %q", removed[0])
	}
}
