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

	lark "github.com/larksuite/oapi-sdk-go/v3"
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

type larkTenantTokenMode string

const (
	larkTenantTokenAuto   larkTenantTokenMode = "auto"
	larkTenantTokenStatic larkTenantTokenMode = "static"
)

type larkAccessToken struct {
	token      string
	kind       larkTokenKind
	tenantMode larkTenantTokenMode
	calendarID string
}

func resolveLarkCalendarAuth(ctx context.Context, callID string) (larkAccessToken, *ports.ToolResult) {
	tenantToken := strings.TrimSpace(shared.LarkTenantTokenFromContext(ctx))
	tenantCalendarID := strings.TrimSpace(shared.LarkTenantCalendarIDFromContext(ctx))
	tenantMode := normalizeTenantTokenMode(shared.LarkTenantTokenModeFromContext(ctx))

	buildTenantAuth := func() (larkAccessToken, *ports.ToolResult) {
		if tenantMode == larkTenantTokenStatic {
			if tenantToken == "" {
				err := fmt.Errorf("tenant token mode is static but channels.lark.tenant_access_token is empty")
				return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
			}
			return larkAccessToken{
				token:      tenantToken,
				kind:       larkTokenTenant,
				tenantMode: tenantMode,
				calendarID: tenantCalendarID,
			}, nil
		}
		return larkAccessToken{
			kind:       larkTokenTenant,
			tenantMode: larkTenantTokenAuto,
			calendarID: tenantCalendarID,
		}, nil
	}

	raw := shared.LarkOAuthFromContext(ctx)
	if raw == nil {
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		err := fmt.Errorf("lark oauth not configured (missing oauth service in context)")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	svc, ok := raw.(larkOAuthService)
	if !ok || svc == nil {
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		err := fmt.Errorf("lark oauth not configured (missing oauth service in context)")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	openID := strings.TrimSpace(id.UserIDFromContext(ctx))
	if openID == "" {
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		err := fmt.Errorf("missing lark sender open_id in context")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
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
		return larkAccessToken{}, &ports.ToolResult{
			CallID:  callID,
			Content: fmt.Sprintf("Failed to load Lark user access token: %v", err),
			Error:   err,
		}
	}

	token = strings.TrimSpace(token)
	if token == "" {
		if auth, errResult := buildTenantAuth(); errResult == nil {
			return auth, nil
		}
		err := fmt.Errorf("empty user_access_token returned")
		return larkAccessToken{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	return larkAccessToken{token: token, kind: larkTokenUser}, nil
}

func buildLarkAuthOptions(auth larkAccessToken) (larkapi.CallOption, larkcore.RequestOptionFunc) {
	if auth.kind == larkTokenTenant {
		if auth.tenantMode == larkTenantTokenStatic {
			// Use user token slot to bypass the SDK tenant-token cache and send the
			// provided tenant token directly as the Authorization header.
			return larkapi.WithUserToken(auth.token), larkcore.WithUserAccessToken(auth.token)
		}
		return larkapi.WithTenantToken(""), larkcore.WithTenantAccessToken("")
	}
	return larkapi.WithUserToken(auth.token), larkcore.WithUserAccessToken(auth.token)
}

func resolveCalendarID(ctx context.Context, callID string, client *lark.Client, auth larkAccessToken, callOpt larkapi.CallOption, toolName string) (string, *ports.ToolResult) {
	if auth.kind == larkTokenTenant {
		calendarID := strings.TrimSpace(auth.calendarID)
		if calendarID == "" {
			err := fmt.Errorf("%s: tenant token mode requires channels.lark.tenant_calendar_id or user OAuth", toolName)
			return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
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

func normalizeTenantTokenMode(mode string) larkTenantTokenMode {
	trimmed := strings.ToLower(strings.TrimSpace(mode))
	switch trimmed {
	case "static":
		return larkTenantTokenStatic
	case "auto", "":
		return larkTenantTokenAuto
	default:
		return larkTenantTokenAuto
	}
}
