package scheduler

import (
	"context"
	"encoding/json"
	"fmt"

	"alex/internal/logging"

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
