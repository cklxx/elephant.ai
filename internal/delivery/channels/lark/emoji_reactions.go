package lark

import (
	"context"
	"strings"
	"time"
)

const defaultProcessingReactEmoji = "OnIt"

// resolveProcessingEmoji returns the configured processing emoji, falling back to default.
func (g *Gateway) resolveProcessingEmoji() string {
	emoji := strings.TrimSpace(g.cfg.ProcessingReactEmoji)
	if emoji == "" {
		return defaultProcessingReactEmoji
	}
	return emoji
}

// addProcessingReaction adds a processing emoji to indicate a task is in progress.
// Returns the reaction ID for later removal, or empty if the reaction could not be added.
func (g *Gateway) addProcessingReaction(ctx context.Context, messageID string) string {
	if messageID == "" {
		return ""
	}
	emoji := g.resolveProcessingEmoji()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return g.addReaction(ctx, messageID, emoji)
}

// removeProcessingReaction removes a previously added processing emoji.
func (g *Gateway) removeProcessingReaction(ctx context.Context, messageID, reactionID string) {
	if messageID == "" || reactionID == "" {
		return
	}
	go func() {
		rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		g.deleteReaction(rmCtx, messageID, reactionID)
	}()
}
