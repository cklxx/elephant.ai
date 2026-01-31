package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/logging"
	"alex/internal/moltbook"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// LarkNotifier sends scheduler results to Lark chats.
type LarkNotifier struct {
	client *lark.Client
	logger logging.Logger
}

// NewLarkNotifier creates a LarkNotifier with the given app credentials.
func NewLarkNotifier(appID, appSecret string, logger logging.Logger) *LarkNotifier {
	return &LarkNotifier{
		client: lark.NewClient(appID, appSecret),
		logger: logging.OrNop(logger),
	}
}

// NewLarkNotifierWithClient creates a LarkNotifier with a pre-built Lark client.
func NewLarkNotifierWithClient(client *lark.Client, logger logging.Logger) *LarkNotifier {
	return &LarkNotifier{
		client: client,
		logger: logging.OrNop(logger),
	}
}

// SendMoltbook is a no-op for Lark-only notifier.
func (n *LarkNotifier) SendMoltbook(_ context.Context, _ string) error {
	return nil
}

// SendLark sends a text message to a Lark chat.
func (n *LarkNotifier) SendLark(ctx context.Context, chatID, content string) error {
	if n.client == nil {
		return fmt.Errorf("lark client not initialized")
	}

	textJSON, err := json.Marshal(map[string]string{"text": content})
	if err != nil {
		return fmt.Errorf("marshal text content: %w", err)
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType("text").
			Content(string(textJSON)).
			Build()).
		Build()

	resp, err := n.client.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("lark send: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("lark send error: code=%d msg=%s", resp.Code, resp.Msg)
	}

	n.logger.Debug("Scheduler: sent Lark message to %s", chatID)
	return nil
}

// NopNotifier is a no-op notifier for testing or when notifications are disabled.
type NopNotifier struct{}

// SendLark is a no-op.
func (NopNotifier) SendLark(_ context.Context, _ string, _ string) error {
	return nil
}

// SendMoltbook is a no-op.
func (NopNotifier) SendMoltbook(_ context.Context, _ string) error {
	return nil
}

// MoltbookNotifier posts scheduler results to Moltbook.
type MoltbookNotifier struct {
	client *moltbook.RateLimitedClient
	logger logging.Logger
}

// NewMoltbookNotifier creates a MoltbookNotifier.
func NewMoltbookNotifier(client *moltbook.RateLimitedClient, logger logging.Logger) *MoltbookNotifier {
	return &MoltbookNotifier{
		client: client,
		logger: logging.OrNop(logger),
	}
}

// SendLark is a no-op for Moltbook-only notifier.
func (n *MoltbookNotifier) SendLark(_ context.Context, _ string, _ string) error {
	return nil
}

// SendMoltbook publishes the content as a Moltbook post.
func (n *MoltbookNotifier) SendMoltbook(ctx context.Context, content string) error {
	title, body := splitTitleBody(content)
	_, err := n.client.CreatePost(ctx, moltbook.CreatePostRequest{
		Title:   title,
		Content: body,
	})
	if err != nil {
		return fmt.Errorf("moltbook notify: %w", err)
	}
	n.logger.Debug("Scheduler: published Moltbook post: %s", title)
	return nil
}

// splitTitleBody splits content into a title (first line) and body (rest).
// If the content is a single line, it is used as both title and body.
func splitTitleBody(content string) (string, string) {
	content = strings.TrimSpace(content)
	if idx := strings.IndexByte(content, '\n'); idx > 0 {
		title := strings.TrimSpace(content[:idx])
		body := strings.TrimSpace(content[idx+1:])
		if title != "" && body != "" {
			return title, body
		}
	}
	// Fallback: truncate long content for title.
	title := content
	if len(title) > 80 {
		title = title[:77] + "..."
	}
	return title, content
}

// CompositeNotifier delegates to multiple notifiers.
type CompositeNotifier struct {
	notifiers []Notifier
}

// NewCompositeNotifier creates a notifier that fans out to all provided notifiers.
func NewCompositeNotifier(notifiers ...Notifier) *CompositeNotifier {
	return &CompositeNotifier{notifiers: notifiers}
}

// SendLark delegates to all notifiers, returning the first error.
func (c *CompositeNotifier) SendLark(ctx context.Context, chatID, content string) error {
	for _, n := range c.notifiers {
		if err := n.SendLark(ctx, chatID, content); err != nil {
			return err
		}
	}
	return nil
}

// SendMoltbook delegates to all notifiers, returning the first error.
func (c *CompositeNotifier) SendMoltbook(ctx context.Context, content string) error {
	for _, n := range c.notifiers {
		if err := n.SendMoltbook(ctx, content); err != nil {
			return err
		}
	}
	return nil
}
