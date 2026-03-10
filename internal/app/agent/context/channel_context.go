package context

import "context"

type channelKey struct{}
type chatIDKey struct{}

// WithChannel annotates the request context with its channel identifier
// (e.g., lark/cli/web).
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


