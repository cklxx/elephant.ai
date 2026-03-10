package lark

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strings"

	builtinshared "alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils"
	id "alex/internal/shared/utils/id"
)

func isInjectSyntheticMessageID(messageID string) bool {
	id := strings.TrimSpace(messageID)
	if id == "" {
		return false
	}
	return strings.HasPrefix(id, "inject_")
}

// newSessionID generates a fresh session identifier for a new Lark task.
func (g *Gateway) newSessionID() string {
	prefix := strings.TrimSpace(g.cfg.SessionPrefix)
	if prefix == "" {
		prefix = "lark"
	}
	return fmt.Sprintf("%s-%s", prefix, id.NewKSUID())
}

// memoryIDForChat derives a deterministic memory identity from a chat ID.
// This stable ID is used as a fallback reset target for the chat.
func (g *Gateway) memoryIDForChat(chatID string) string {
	hash := sha1.Sum([]byte(chatID))
	return fmt.Sprintf("%s-%x", g.cfg.SessionPrefix, hash[:12])
}

// withLarkContext injects the common Lark tool context values (client, chat ID,
// message ID, and base domain) into the given context. All call sites that
// previously called WithLarkClient + WithLarkChatID + WithLarkMessageID
// individually should use this helper instead to ensure BaseDomain is always set.
func (g *Gateway) withLarkContext(ctx context.Context, chatID, messageID string) context.Context {
	ctx = builtinshared.WithLarkClient(ctx, g.client)
	ctx = builtinshared.WithLarkMessenger(ctx, g.messenger)
	ctx = builtinshared.WithLarkChatID(ctx, chatID)
	ctx = builtinshared.WithLarkMessageID(ctx, messageID)
	if domain := strings.TrimSpace(g.cfg.BaseDomain); domain != "" {
		ctx = builtinshared.WithLarkBaseDomain(ctx, domain)
	}
	return ctx
}

// deref safely dereferences a string pointer.
func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func normalizeExtensions(exts []string) []string {
	if len(exts) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(exts))
	normalized := make([]string, 0, len(exts))
	for _, raw := range exts {
		trimmed := utils.TrimLower(raw)
		if trimmed == "" {
			continue
		}
		if !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
