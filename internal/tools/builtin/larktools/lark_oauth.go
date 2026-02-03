package larktools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	larkapi "alex/internal/lark"
	larkoauth "alex/internal/lark/oauth"
	"alex/internal/tools/builtin/shared"
	"alex/internal/utils/id"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

type larkOAuthService interface {
	UserAccessToken(ctx context.Context, openID string) (string, error)
	StartURL() string
}

type larkTokenKind uint8

const (
	larkTokenUser larkTokenKind = iota
	larkTokenTenant
)

type larkAccessToken struct {
	token string
	kind  larkTokenKind
}

func resolveLarkCalendarAuth(ctx context.Context, callID string) (larkAccessToken, *ports.ToolResult) {
	tenantToken := strings.TrimSpace(shared.LarkTenantTokenFromContext(ctx))
	raw := shared.LarkOAuthFromContext(ctx)
	if raw == nil {
		if tenantToken != "" {
			return larkAccessToken{token: tenantToken, kind: larkTokenTenant}, nil
		}
		err := fmt.Errorf("lark oauth not configured (missing oauth service in context)")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	svc, ok := raw.(larkOAuthService)
	if !ok || svc == nil {
		if tenantToken != "" {
			return larkAccessToken{token: tenantToken, kind: larkTokenTenant}, nil
		}
		err := fmt.Errorf("lark oauth not configured (missing oauth service in context)")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	openID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if openID == "" {
		if tenantToken != "" {
			return larkAccessToken{token: tenantToken, kind: larkTokenTenant}, nil
		}
		err := fmt.Errorf("missing lark sender open_id in context")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	token, err := svc.UserAccessToken(ctx, openID)
	if err != nil {
		var need *larkoauth.NeedUserAuthError
		if errors.As(err, &need) {
			if tenantToken != "" {
				return larkAccessToken{token: tenantToken, kind: larkTokenTenant}, nil
			}
			url := strings.TrimSpace(need.AuthURL)
			if url == "" {
				url = strings.TrimSpace(svc.StartURL())
			}
			if url == "" {
				url = "(missing auth url)"
			}
			return larkAccessToken{}, &ports.ToolResult{
				CallID:  callID,
				Content: fmt.Sprintf("Please authorize Lark calendar access first: %s", url),
				Error:   err,
			}
		}
		return larkAccessToken{}, &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("Failed to load Lark user access token: %v", err),
			Error:   err,
		}
	}

	token = strings.TrimSpace(token)
	if token == "" {
		if tenantToken != "" {
			return larkAccessToken{token: tenantToken, kind: larkTokenTenant}, nil
		}
		err := fmt.Errorf("empty user_access_token returned")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	return larkAccessToken{token: token, kind: larkTokenUser}, nil
}

func buildLarkAuthOptions(auth larkAccessToken) (larkapi.CallOption, larkcore.RequestOptionFunc) {
	if auth.kind == larkTokenTenant {
		// Use user token slot to bypass the SDK tenant-token cache and send the
		// provided tenant token directly as the Authorization header.
		return larkapi.WithUserToken(auth.token), larkcore.WithUserAccessToken(auth.token)
	}
	return larkapi.WithUserToken(auth.token), larkcore.WithUserAccessToken(auth.token)
}
