package context

import "context"

type channelKey struct{}
type chatIDKey struct{}
type isGroupKey struct{}

// WithChannel annotates the request context with its channel identifier
// (e.g., lark/wechat/cli/web).
func WithChannel(ctx context.Context, channel string) context.Context {
	return context.WithValue(ctx, channelKey{}, channel)
}

// ChannelFromContext returns the channel identifier from context.
func ChannelFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if channel, ok := ctx.Value(channelKey{}).(string); ok {
		return channel
	}
	return ""
}

// WithChatID annotates the request context with the chat/conversation ID.
func WithChatID(ctx context.Context, chatID string) context.Context {
	return context.WithValue(ctx, chatIDKey{}, chatID)
}

// ChatIDFromContext returns the chat/conversation ID from context.
func ChatIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if chatID, ok := ctx.Value(chatIDKey{}).(string); ok {
		return chatID
	}
	return ""
}

// WithIsGroup annotates whether the request originates from a group chat.
func WithIsGroup(ctx context.Context, isGroup bool) context.Context {
	return context.WithValue(ctx, isGroupKey{}, isGroup)
}

// IsGroupFromContext returns true when the request is marked as a group chat.
func IsGroupFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if isGroup, ok := ctx.Value(isGroupKey{}).(bool); ok {
		return isGroup
	}
	return false
}
