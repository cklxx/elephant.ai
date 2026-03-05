package larktools

import (
	"context"
	"log/slog"

	larkapi "alex/internal/infra/lark"
	id "alex/internal/shared/utils/id"
)

// grantSenderEditPermission grants the current message sender edit permission
// on a newly created document/spreadsheet/folder. Failures are logged but do
// not block the creation result — the document is still usable via the bot.
func grantSenderEditPermission(ctx context.Context, client *larkapi.Client, token, docType string) {
	senderID := id.UserIDFromContext(ctx)
	if senderID == "" {
		return
	}

	err := client.Permission().GrantPermission(ctx, larkapi.GrantPermissionRequest{
		Token:      token,
		Type:       docType,
		MemberID:   senderID,
		MemberType: "openid",
		Perm:       "full_access",
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to grant sender edit permission",
			"token", token, "type", docType, "sender", senderID, "error", err)
	}
}
