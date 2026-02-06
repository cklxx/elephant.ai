package artifacts

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestA2UIEmitStoresAttachment(t *testing.T) {
	tool := NewA2UIEmit()
	payload := `{"beginRendering":{"surfaceId":"main","root":"root"}}`
	call := ports.ToolCall{
		ID:   "call-1",
		Name: "a2ui_emit",
		Arguments: map[string]any{
			"content": payload,
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if len(result.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(result.Attachments))
	}

	var att ports.Attachment
	for _, entry := range result.Attachments {
		att = entry
		break
	}
	if att.MediaType != "application/a2ui+json" {
		t.Fatalf("unexpected media type: %s", att.MediaType)
	}
	if att.Format != "a2ui" {
		t.Fatalf("unexpected format: %s", att.Format)
	}
	if att.PreviewProfile != "document.a2ui" {
		t.Fatalf("unexpected preview profile: %s", att.PreviewProfile)
	}
	if !strings.HasSuffix(att.Name, ".jsonl") {
		t.Fatalf("expected jsonl filename, got %s", att.Name)
	}
	decoded, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Fatalf("failed to decode attachment data: %v", err)
	}
	if string(decoded) != payload {
		t.Fatalf("unexpected payload: %s", string(decoded))
	}

	mutationsRaw, ok := result.Metadata["attachment_mutations"]
	if !ok || mutationsRaw == nil {
		t.Fatalf("expected attachment mutations metadata, got: %+v", result.Metadata)
	}
	mutations, ok := mutationsRaw.(map[string]any)
	if !ok {
		t.Fatalf("unexpected mutations type: %T", mutationsRaw)
	}
	if _, ok := mutations["add"]; !ok {
		t.Fatalf("expected add mutation, got: %+v", mutations)
	}
}

func TestA2UIEmitSerializesMessages(t *testing.T) {
	tool := NewA2UIEmit()
	call := ports.ToolCall{
		ID:   "call-2",
		Name: "a2ui_emit",
		Arguments: map[string]any{
			"messages": []any{
				map[string]any{
					"beginRendering": map[string]any{
						"surfaceId": "main",
						"root":      "root",
					},
				},
				map[string]any{
					"dataModelUpdate": map[string]any{
						"surfaceId": "main",
						"contents":  []any{},
					},
				},
			},
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}

	var att ports.Attachment
	for _, entry := range result.Attachments {
		att = entry
		break
	}
	decoded, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Fatalf("failed to decode attachment data: %v", err)
	}
	decodedStr := string(decoded)
	if !strings.Contains(decodedStr, "\"beginRendering\"") || !strings.Contains(decodedStr, "\"dataModelUpdate\"") {
		t.Fatalf("expected serialized messages, got: %s", decodedStr)
	}
	if !strings.Contains(decodedStr, "\n") {
		t.Fatalf("expected JSONL output with newline, got: %s", decodedStr)
	}
}

func TestA2UIEmitSerializesContentObject(t *testing.T) {
	tool := NewA2UIEmit()
	call := ports.ToolCall{
		ID:   "call-3",
		Name: "a2ui_emit",
		Arguments: map[string]any{
			"content": map[string]any{
				"type":    "ui",
				"version": "1.0",
				"messages": []any{
					map[string]any{
						"type": "heading",
						"text": "Hello",
					},
				},
			},
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}

	var att ports.Attachment
	for _, entry := range result.Attachments {
		att = entry
		break
	}
	decoded, err := base64.StdEncoding.DecodeString(att.Data)
	if err != nil {
		t.Fatalf("failed to decode attachment data: %v", err)
	}
	if !json.Valid(decoded) {
		t.Fatalf("expected JSON content, got: %s", string(decoded))
	}
	if !strings.Contains(string(decoded), "\"messages\"") {
		t.Fatalf("expected serialized messages, got: %s", string(decoded))
	}
}

func TestA2UIEmitDefinitionMessagesHasItems(t *testing.T) {
	def := NewA2UIEmit().Definition()

	raw, err := json.Marshal(def.Parameters)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}

	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("unmarshal schema: %v", err)
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected properties object in schema")
	}
	messages, ok := properties["messages"].(map[string]any)
	if !ok {
		t.Fatalf("expected messages schema object")
	}
	if _, ok := messages["items"]; !ok {
		t.Fatalf("expected messages array schema to include items")
	}
}
