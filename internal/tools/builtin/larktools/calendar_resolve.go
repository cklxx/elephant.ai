package larktools

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	larkapi "alex/internal/lark"
	"alex/internal/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

func resolveCalendarID(ctx context.Context, client *lark.Client, callID, calendarID string, args map[string]any) (string, *ports.ToolResult) {
	trimmed := strings.TrimSpace(calendarID)
	if trimmed == "" {
		return "", &ports.ToolResult{
			CallID:  callID,
			Content: "missing 'calendar_id'",
			Error:   fmt.Errorf("missing 'calendar_id'"),
		}
	}
	if !strings.EqualFold(trimmed, "primary") {
		return trimmed, nil
	}

	token := shared.StringArg(args, "user_access_token")
	resolved, err := larkapi.Wrap(client).Calendar().ResolveCalendarID(ctx, trimmed, larkapi.WithUserToken(token))
	if err != nil {
		return "", &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("failed to resolve calendar_id=%q: %v", trimmed, err),
			Error:   fmt.Errorf("resolve calendar id: %w", err),
		}
	}
	return resolved, nil
}
