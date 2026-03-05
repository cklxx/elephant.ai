package larktools

import (
	"context"
	"fmt"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// requireLarkClient extracts and validates the *lark.Client from context.
// Returns a non-nil *ports.ToolResult on failure (caller should return it).
func requireLarkClient(ctx context.Context, callID string) (*lark.Client, *ports.ToolResult) {
	raw := shared.LarkClientFromContext(ctx)
	if raw == nil {
		return nil, &ports.ToolResult{
			CallID:  callID,
			Content: "This operation requires a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}
	}
	client, ok := raw.(*lark.Client)
	if !ok {
		return nil, &ports.ToolResult{
			CallID:  callID,
			Content: "Invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", raw),
		}
	}
	return client, nil
}
