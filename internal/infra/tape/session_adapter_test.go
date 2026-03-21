package tape

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestMessageRoundTrip_AllFields(t *testing.T) {
	msg := ports.Message{
		Role:       "assistant",
		Content:    "hello world",
		Source:     ports.MessageSourceAssistantReply,
		ToolCallID: "tc_123",
		ToolCalls: []ports.ToolCall{
			{ID: "tc_1", Name: "search", Arguments: map[string]any{"q": "weather"}},
		},
		ToolResults: []ports.ToolResult{
			{CallID: "tc_1", Content: "sunny"},
		},
		Thinking: ports.Thinking{
			Parts: []ports.ThinkingPart{
				{Kind: "thinking", Text: "let me think about this"},
				{Kind: "redacted", Encrypted: "enc_data", Signature: "sig_data"},
			},
		},
		Metadata: map[string]any{
			"context_placeholder": true,
			"tokens_removed":      42,
		},
		Attachments: map[string]ports.Attachment{
			"img.png": {Name: "img.png", MediaType: "image/png", Data: "base64data"},
		},
	}

	entry := messageToEntry(msg, "sess_test")
	got, err := entryToMessage(entry)
	if err != nil {
		t.Fatalf("entryToMessage failed: %v", err)
	}

	assertEqual(t, "Role", got.Role, msg.Role)
	assertEqual(t, "Content", got.Content, msg.Content)
	assertEqual(t, "Source", string(got.Source), string(msg.Source))
	assertEqual(t, "ToolCallID", got.ToolCallID, msg.ToolCallID)

	if len(got.ToolCalls) != 1 {
		t.Fatalf("ToolCalls: got %d, want 1", len(got.ToolCalls))
	}
	assertEqual(t, "ToolCalls[0].Name", got.ToolCalls[0].Name, "search")

	if len(got.ToolResults) != 1 {
		t.Fatalf("ToolResults: got %d, want 1", len(got.ToolResults))
	}
	assertEqual(t, "ToolResults[0].Content", got.ToolResults[0].Content, "sunny")

	if len(got.Thinking.Parts) != 2 {
		t.Fatalf("Thinking.Parts: got %d, want 2", len(got.Thinking.Parts))
	}
	assertEqual(t, "Thinking[0].Text", got.Thinking.Parts[0].Text, "let me think about this")
	assertEqual(t, "Thinking[1].Encrypted", got.Thinking.Parts[1].Encrypted, "enc_data")

	if got.Metadata == nil {
		t.Fatal("Metadata is nil after round-trip")
	}
	if v, ok := got.Metadata["context_placeholder"].(bool); !ok || !v {
		t.Fatalf("Metadata[context_placeholder]: got %v", got.Metadata["context_placeholder"])
	}

	if len(got.Attachments) != 1 {
		t.Fatalf("Attachments: got %d, want 1", len(got.Attachments))
	}
	if att, ok := got.Attachments["img.png"]; !ok || att.MediaType != "image/png" {
		t.Fatalf("Attachments[img.png]: got %+v", got.Attachments)
	}
}

func TestMessageRoundTrip_MinimalMessage(t *testing.T) {
	msg := ports.Message{Role: "user", Content: "hi"}
	entry := messageToEntry(msg, "sess_1")
	got, err := entryToMessage(entry)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "Role", got.Role, "user")
	assertEqual(t, "Content", got.Content, "hi")
	if len(got.Thinking.Parts) != 0 {
		t.Fatal("expected no Thinking parts")
	}
	if len(got.Metadata) != 0 {
		t.Fatal("expected no Metadata")
	}
}

func TestSessionAdapter_SaveAndGet_PreservesThinking(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	adapter := NewSessionAdapter(store)

	sess, err := adapter.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}

	sess.Messages = append(sess.Messages, ports.Message{
		Role:    "assistant",
		Content: "reasoning result",
		Source:  ports.MessageSourceAssistantReply,
		Thinking: ports.Thinking{
			Parts: []ports.ThinkingPart{
				{Kind: "thinking", Text: "step 1: analyze"},
			},
		},
		Metadata: map[string]any{"turn": 1},
	})

	if err := adapter.Save(ctx, sess); err != nil {
		t.Fatal(err)
	}

	loaded, err := adapter.Get(ctx, sess.ID)
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded.Messages) != 1 {
		t.Fatalf("Messages: got %d, want 1", len(loaded.Messages))
	}
	got := loaded.Messages[0]
	if len(got.Thinking.Parts) != 1 {
		t.Fatalf("Thinking.Parts: got %d, want 1", len(got.Thinking.Parts))
	}
	assertEqual(t, "Thinking[0].Text", got.Thinking.Parts[0].Text, "step 1: analyze")
	if got.Metadata == nil {
		t.Fatal("Metadata lost after Save+Get")
	}
}

func assertEqual[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("%s = %v, want %v", field, got, want)
	}
}
