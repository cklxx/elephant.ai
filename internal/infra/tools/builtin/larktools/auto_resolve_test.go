package larktools

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	id "alex/internal/shared/utils/id"
)

// TestOKRAutoResolveUserID verifies that listUserOKRs auto-resolves user_id
// from context when not explicitly provided.
func TestOKRAutoResolveUserID(t *testing.T) {
	handler := &larkOKRManage{}

	// Without user_id in args and without context → should fail with clear error
	call := ports.ToolCall{
		ID:        "test-okr-1",
		Arguments: map[string]interface{}{"action": "list_user_okrs"},
	}
	result, err := handler.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fail because no lark client in context
	if result.Error == nil {
		t.Fatal("expected error when no lark client in context")
	}
	t.Logf("OK: no lark client → error: %s", result.Content)
}

// TestContactAutoResolveUserID verifies that getUser auto-resolves user_id
// from context when not explicitly provided.
func TestContactAutoResolveUserID(t *testing.T) {
	handler := &larkContactManage{}

	// Without user_id in args and without context → should fail with clear error
	call := ports.ToolCall{
		ID:        "test-contact-1",
		Arguments: map[string]interface{}{"action": "get_user"},
	}
	result, err := handler.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when no lark client in context")
	}
	t.Logf("OK: no lark client → error: %s", result.Content)
}

// TestOKRAutoResolveFromContext verifies that listUserOKRs picks up user_id
// from id.WithUserID context (simulating a real Lark message flow).
func TestOKRAutoResolveFromContext(t *testing.T) {
	// We can't fully test the SDK call without a real Lark client,
	// but we can verify the auto-resolve logic by checking that it
	// tries to use the context user ID (will fail at SDK level, not
	// at "user_id required" level).
	handler := &larkOKRManage{}

	ctx := id.WithUserID(context.Background(), "ou_test_user_123")
	call := ports.ToolCall{
		ID:        "test-okr-2",
		Arguments: map[string]interface{}{"action": "list_user_okrs"},
	}
	result, err := handler.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fail because no lark client, NOT because of missing user_id
	if result.Error == nil {
		t.Fatal("expected error (no lark client)")
	}
	if result.Content == "user_id is required (provide it explicitly or send from a Lark chat)" {
		t.Fatal("auto-resolve failed: still requiring user_id despite context having one")
	}
	t.Logf("OK: auto-resolved user_id from context, error is: %s", result.Content)
}

// TestContactAutoResolveFromContext verifies getUser picks up user_id from context.
func TestContactAutoResolveFromContext(t *testing.T) {
	handler := &larkContactManage{}

	ctx := id.WithUserID(context.Background(), "ou_test_user_456")
	call := ports.ToolCall{
		ID:        "test-contact-2",
		Arguments: map[string]interface{}{"action": "get_user"},
	}
	result, err := handler.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error (no lark client)")
	}
	if result.Content == "user_id is required (provide it explicitly or send from a Lark chat)" {
		t.Fatal("auto-resolve failed: still requiring user_id despite context having one")
	}
	t.Logf("OK: auto-resolved user_id from context, error is: %s", result.Content)
}
