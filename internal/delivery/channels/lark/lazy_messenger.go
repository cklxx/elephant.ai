package lark

import (
	"context"
	"fmt"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// lazyMessenger wraps a Gateway and delegates all LarkMessenger calls to the
// gateway's internal messenger. This allows AutoAuth to be configured before
// the gateway's messenger is initialized (which happens during Start()).
type lazyMessenger struct {
	g *Gateway
}

func (l *lazyMessenger) get() (LarkMessenger, error) {
	if l.g == nil || l.g.messenger == nil {
		return nil, fmt.Errorf("lark messenger not yet initialized")
	}
	return l.g.messenger, nil
}

func (l *lazyMessenger) SendMessage(ctx context.Context, chatID, msgType, content string) (string, error) {
	m, err := l.get()
	if err != nil {
		return "", err
	}
	return m.SendMessage(ctx, chatID, msgType, content)
}

func (l *lazyMessenger) ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error) {
	m, err := l.get()
	if err != nil {
		return "", err
	}
	return m.ReplyMessage(ctx, replyToID, msgType, content)
}

func (l *lazyMessenger) UpdateMessage(ctx context.Context, messageID, msgType, content string) error {
	m, err := l.get()
	if err != nil {
		return err
	}
	return m.UpdateMessage(ctx, messageID, msgType, content)
}

func (l *lazyMessenger) AddReaction(ctx context.Context, messageID, emojiType string) (string, error) {
	m, err := l.get()
	if err != nil {
		return "", err
	}
	return m.AddReaction(ctx, messageID, emojiType)
}

func (l *lazyMessenger) DeleteReaction(ctx context.Context, messageID, reactionID string) error {
	m, err := l.get()
	if err != nil {
		return err
	}
	return m.DeleteReaction(ctx, messageID, reactionID)
}

func (l *lazyMessenger) UploadImage(ctx context.Context, payload []byte) (string, error) {
	m, err := l.get()
	if err != nil {
		return "", err
	}
	return m.UploadImage(ctx, payload)
}

func (l *lazyMessenger) UploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	m, err := l.get()
	if err != nil {
		return "", err
	}
	return m.UploadFile(ctx, payload, fileName, fileType)
}

func (l *lazyMessenger) ListMessages(ctx context.Context, chatID string, pageSize int) ([]*larkim.Message, error) {
	m, err := l.get()
	if err != nil {
		return nil, err
	}
	return m.ListMessages(ctx, chatID, pageSize)
}

var _ LarkMessenger = (*lazyMessenger)(nil)
