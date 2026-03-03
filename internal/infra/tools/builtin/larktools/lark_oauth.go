package larktools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkapi "alex/internal/infra/lark"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils/id"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

type larkTokenKind uint8

const (
	larkTokenUser larkTokenKind = iota
	larkTokenTenant
)

type larkAccessToken struct {
	token      string
	kind       larkTokenKind
	calendarID string
}

// resolveLarkTaskAuth resolves task API auth with user token priority and
// tenant fallback.
//
// Fallback behavior follows task-v2 guidance: tenant_access_token has no
// elevated privilege, but it is a valid app identity and should keep API calls
// available when user OAuth is not ready.
func resolveLarkTaskAuth(ctx context.Context) larkAccessToken {
	svc := shared.LarkOAuthFromContext(ctx)
	if svc == nil {
		return larkAccessToken{kind: larkTokenTenant}
	}

	openID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if openID == "" {
		return larkAccessToken{kind: larkTokenTenant}
	}

	token, err := svc.UserAccessToken(ctx, openID)
	if err != nil {
		return larkAccessToken{kind: larkTokenTenant}
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return larkAccessToken{kind: larkTokenTenant}
	}

	return larkAccessToken{token: token, kind: larkTokenUser}
}

func resolveLarkCalendarAuth(ctx context.Context, callID string) (larkAccessToken, *ports.ToolResult) {
	tenantCalendarID := strings.TrimSpace(shared.LarkTenantCalendarIDFromContext(ctx))

	buildTenantAuth := func() (larkAccessToken, *ports.ToolResult) {
		return larkAccessToken{
			kind:       larkTokenTenant,
			calendarID: tenantCalendarID,
		}, nil
	}

	svc := shared.LarkOAuthFromContext(ctx)
	if svc == nil {
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		result, _ := shared.ToolError(callID, "lark oauth not configured (missing oauth service in context)")
		return larkAccessToken{}, result
	}

	openID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if openID == "" {
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		result, _ := shared.ToolError(callID, "missing lark sender open_id in context")
		return larkAccessToken{}, result
	}

	token, err := svc.UserAccessToken(ctx, openID)
	if err != nil {
		var need *larkoauth.NeedUserAuthError
		if errors.As(err, &need) {
			if auth, errResult := buildTenantAuth(); errResult == nil {
				return auth, nil
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
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		return larkAccessToken{}, apiErr(callID, "load Lark user access token", err)
	}

	token = strings.TrimSpace(token)
	if token == "" {
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		result, _ := shared.ToolError(callID, "empty user_access_token returned")
		return larkAccessToken{}, result
	}

	return larkAccessToken{token: token, kind: larkTokenUser}, nil
}

func buildLarkAuthOptions(auth larkAccessToken) (larkapi.CallOption, larkcore.RequestOptionFunc) {
	if auth.kind == larkTokenTenant {
		return larkapi.WithTenantToken(""), larkcore.WithTenantAccessToken("")
	}
	return larkapi.WithUserToken(auth.token), larkcore.WithUserAccessToken(auth.token)
}

func resolveCalendarID(ctx context.Context, callID string, client *lark.Client, auth larkAccessToken, callOpt larkapi.CallOption, toolName string) (string, *ports.ToolResult) {
	if auth.kind == larkTokenTenant {
		calendarID := strings.TrimSpace(auth.calendarID)
		if calendarID == "" {
			result, _ := shared.ToolError(callID, "%s: tenant token requires channels.lark.tenant_calendar_id or user OAuth", toolName)
			return "", result
		}
		return calendarID, nil
	}

	calendarID, err := larkapi.Wrap(client).Calendar().ResolveCalendarID(ctx, "primary", callOpt)
	if err != nil {
		return "", &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("%s: failed to resolve primary calendar_id: %v", toolName, err),
			Error:   fmt.Errorf("resolve primary calendar id: %w", err),
		}
	}
	return calendarID, nil
}

func toolErrorResult(callID, format string, args ...any) *ports.ToolResult {
	result, _ := shared.ToolError(callID, format, args...)
	return result
}
