// Package notification provides channel-specific notification senders that
// implement the shared notification.Notifier interface.
package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/infra/moltbook"
	"alex/internal/shared/logging"
	"alex/internal/shared/notification"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// LarkSender sends notifications to Lark chats.
type LarkSender struct {
	client *lark.Client
	logger logging.Logger
}

// NewLarkSender creates a LarkSender with the given app credentials.
func NewLarkSender(appID, appSecret string, logger logging.Logger) *LarkSender {
	return &LarkSender{
		client: lark.NewClient(appID, appSecret),
		logger: logging.OrNop(logger),
	}
}

// NewLarkSenderWithClient creates a LarkSender with a pre-built Lark client.
func NewLarkSenderWithClient(client *lark.Client, logger logging.Logger) *LarkSender {
	return &LarkSender{
		client: client,
		logger: logging.OrNop(logger),
	}
}

// Send sends a notification. Only handles channel "lark".
func (n *LarkSender) Send(ctx context.Context, target notification.Target, content string) error {
	if target.Channel != "lark" {
		return nil
	}
	if target.ChatID == "" {
		return nil
	}
	return n.sendLark(ctx, target.ChatID, content)
}

// SendLark sends a text message to a Lark chat. Exported for direct use in
// kernel notice pipeline where only Lark sending is needed.
func (n *LarkSender) SendLark(ctx context.Context, chatID, content string) error {
	return n.sendLark(ctx, chatID, content)
}

func (n *LarkSender) sendLark(ctx context.Context, chatID, content string) error {
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

	n.logger.Debug("Notification: sent Lark message to %s", chatID)
	return nil
}

// MoltbookSender posts notifications to Moltbook.
type MoltbookSender struct {
	client *moltbook.RateLimitedClient
	logger logging.Logger
}

// NewMoltbookSender creates a MoltbookSender.
func NewMoltbookSender(client *moltbook.RateLimitedClient, logger logging.Logger) *MoltbookSender {
	return &MoltbookSender{
		client: client,
		logger: logging.OrNop(logger),
	}
}

// Send sends a notification. Only handles channel "moltbook".
func (n *MoltbookSender) Send(ctx context.Context, target notification.Target, content string) error {
	if target.Channel != "moltbook" {
		return nil
	}
	title, body := splitTitleBody(content)
	_, err := n.client.CreatePost(ctx, moltbook.CreatePostRequest{
		Title:   title,
		Content: body,
	})
	if err != nil {
		return fmt.Errorf("moltbook notify: %w", err)
	}
	n.logger.Debug("Notification: published Moltbook post: %s", title)
	return nil
}

// NopNotifier is a no-op notifier for testing or when notifications are disabled.
type NopNotifier struct{}

// Send is a no-op.
func (NopNotifier) Send(_ context.Context, _ notification.Target, _ string) error {
	return nil
}

// CompositeNotifier delegates to multiple notifiers.
type CompositeNotifier struct {
	notifiers []notification.Notifier
}

// NewCompositeNotifier creates a notifier that fans out to all provided notifiers.
func NewCompositeNotifier(notifiers ...notification.Notifier) *CompositeNotifier {
	return &CompositeNotifier{notifiers: notifiers}
}

// Send delegates to all notifiers, returning the first error.
func (c *CompositeNotifier) Send(ctx context.Context, target notification.Target, content string) error {
	for _, n := range c.notifiers {
		if err := n.Send(ctx, target, content); err != nil {
			return err
		}
	}
	return nil
}

// splitTitleBody splits content into a title (first line) and body (rest).
func splitTitleBody(content string) (string, string) {
	content = strings.TrimSpace(content)
	if idx := strings.IndexByte(content, '\n'); idx > 0 {
		title := strings.TrimSpace(content[:idx])
		body := strings.TrimSpace(content[idx+1:])
		if title != "" && body != "" {
			return title, body
		}
	}
	title := content
	if len(title) > 80 {
		title = title[:77] + "..."
	}
	return title, content
}
