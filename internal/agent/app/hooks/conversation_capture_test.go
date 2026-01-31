package hooks

import (
	"context"
	"testing"

	appcontext "alex/internal/agent/app/context"
)

func TestConversationCaptureHook_SkipsWithToolCalls(t *testing.T) {
	svc := &mockMemoryService{}
	hook := NewConversationCaptureHook(svc, nil, ConversationCaptureConfig{})

	ctx := context.Background()
	ctx = appcontext.WithMemoryPolicy(ctx, appcontext.MemoryPolicy{
		Enabled:         true,
		AutoCapture:     true,
		CaptureMessages: true,
	})

	err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "hello",
		Answer:    "world",
		UserID:    "ou_user",
		ToolCalls: []ToolResultInfo{{ToolName: "bash", Success: true}},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if svc.saveCalled != 0 {
		t.Fatalf("expected no save when tool calls present, got %d", svc.saveCalled)
	}
}

func TestConversationCaptureHook_CapturesUserAndGroupScope(t *testing.T) {
	svc := &mockMemoryService{}
	hook := NewConversationCaptureHook(svc, nil, ConversationCaptureConfig{
		CaptureGroupMemory: true,
	})

	ctx := context.Background()
	ctx = appcontext.WithChannel(ctx, "lark")
	ctx = appcontext.WithChatID(ctx, "oc_chat_123")
	ctx = appcontext.WithIsGroup(ctx, true)
	ctx = appcontext.WithMemoryPolicy(ctx, appcontext.MemoryPolicy{
		Enabled:         true,
		AutoCapture:     true,
		CaptureMessages: true,
	})

	err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "hello",
		Answer:    "hi there",
		UserID:    "ou_user",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if svc.saveCalled != 2 {
		t.Fatalf("expected 2 saves (user + chat scope), got %d", svc.saveCalled)
	}

	if svc.entries[0].UserID != "ou_user" {
		t.Fatalf("expected user entry user_id=ou_user, got %q", svc.entries[0].UserID)
	}
	if svc.entries[0].Slots["scope"] != "user" {
		t.Fatalf("expected user scope, got %q", svc.entries[0].Slots["scope"])
	}

	if svc.entries[1].UserID != "chat:lark:oc_chat_123" {
		t.Fatalf("expected chat scope user_id, got %q", svc.entries[1].UserID)
	}
	if svc.entries[1].Slots["scope"] != "chat" {
		t.Fatalf("expected chat scope, got %q", svc.entries[1].Slots["scope"])
	}
	if svc.entries[1].Slots["chat_id"] != "oc_chat_123" {
		t.Fatalf("expected chat_id slot, got %q", svc.entries[1].Slots["chat_id"])
	}
}

func TestConversationCaptureHook_SkipsWhenPolicyDisabled(t *testing.T) {
	svc := &mockMemoryService{}
	hook := NewConversationCaptureHook(svc, nil, ConversationCaptureConfig{})

	ctx := context.Background()
	ctx = appcontext.WithMemoryPolicy(ctx, appcontext.MemoryPolicy{
		Enabled:         false,
		AutoCapture:     false,
		CaptureMessages: false,
	})
	err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "test",
		Answer:    "ok",
		UserID:    "ou_user",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if svc.saveCalled != 0 {
		t.Fatalf("expected no save when disabled, got %d", svc.saveCalled)
	}
}
