package larktools

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	larkapi "alex/internal/lark"
	"alex/internal/tools/builtin/shared"
	"alex/internal/utils/id"

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

	ownerIDType := strings.TrimSpace(shared.StringArg(args, "calendar_owner_id_type"))
	if ownerIDType == "" {
		ownerIDType = "open_id"
	}
	ownerID := strings.TrimSpace(shared.StringArg(args, "calendar_owner_id"))
	if ownerID == "" {
		// Only use ctx user_id as a calendar owner if it looks like a Lark ID.
		// Scheduler/other channels may set user_id to an internal identifier.
		ctxUserID := strings.TrimSpace(id.UserIDFromContext(ctx))
		switch ownerIDType {
		case "open_id":
			if strings.HasPrefix(ctxUserID, "ou_") {
				ownerID = ctxUserID
			}
		case "union_id":
			if strings.HasPrefix(ctxUserID, "on_") {
				ownerID = ctxUserID
			}
		case "user_id":
			if strings.HasPrefix(ctxUserID, "u_") {
				ownerID = ctxUserID
			}
		}
	}

	// If we know who "primary" refers to, resolve it via Primarys(user_ids) first.
	// This enables booking an @mentioned user's calendar without requiring that
	// user's access token.
	if ownerID != "" {
		calSvc := larkapi.Wrap(client).Calendar()
		resolved, err := calSvc.PrimaryCalendarID(ctx, ownerIDType, ownerID)
		if err != nil {
			// Fallback: when resolving the caller's own calendar and a user token
			// is supplied, try user-scoped resolution.
			token := strings.TrimSpace(shared.StringArg(args, "user_access_token"))
			if token != "" && ownerID == strings.TrimSpace(id.UserIDFromContext(ctx)) {
				if viaUser, uErr := calSvc.PrimaryCalendarID(ctx, ownerIDType, ownerID, larkapi.WithUserToken(token)); uErr == nil {
					return viaUser, nil
				}
			}
			return "", &ports.ToolResult{
				CallID:  callID,
				Content: fmt.Sprintf("failed to resolve primary calendar for user_id=%q (type=%s): %v", ownerID, ownerIDType, err),
				Error:   fmt.Errorf("resolve primary calendar id: %w", err),
			}
		}
		return resolved, nil
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
