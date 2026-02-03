package larktools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	larkoauth "alex/internal/lark/oauth"
	"alex/internal/tools/builtin/shared"
	"alex/internal/utils/id"
)

type larkOAuthService interface {
	UserAccessToken(ctx context.Context, openID string) (string, error)
	StartURL() string
}

func requireLarkUserAccessToken(ctx context.Context, callID string) (string, *ports.ToolResult) {
	openID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if openID == "" {
		err := fmt.Errorf("missing lark sender open_id in context")
		return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	raw := shared.LarkOAuthFromContext(ctx)
	svc, ok := raw.(larkOAuthService)
	if !ok || svc == nil {
		err := fmt.Errorf("lark oauth not configured (missing oauth service in context)")
		return "", &ports.ToolResult{
			CallID:  callID,
			Content: err.Error(),
			Error:   err,
		}
	}

	token, err := svc.UserAccessToken(ctx, openID)
	if err != nil {
		var need *larkoauth.NeedUserAuthError
		if errors.As(err, &need) {
			url := strings.TrimSpace(need.AuthURL)
			if url == "" {
				url = strings.TrimSpace(svc.StartURL())
			}
			if url == "" {
				url = "(missing auth url)"
			}
			return "", &ports.ToolResult{
				CallID:  callID,
				Content: fmt.Sprintf("Please authorize Lark calendar access first: %s", url),
				Error:   err,
			}
		}
		return "", &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("Failed to load Lark user access token: %v", err),
			Error:   err,
		}
	}

	token = strings.TrimSpace(token)
	if token == "" {
		err := fmt.Errorf("empty user_access_token returned")
		return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	return token, nil
}
