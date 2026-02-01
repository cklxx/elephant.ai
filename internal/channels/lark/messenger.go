package lark

import (
	"context"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// LarkMessenger abstracts all outbound Lark API operations for testability.
// Production code uses sdkMessenger (wrapping the real Lark SDK client);
// tests use RecordingMessenger to capture and assert all outbound calls.
type LarkMessenger interface {
	// SendMessage creates a new message in a chat.
	SendMessage(ctx context.Context, chatID, msgType, content string) (messageID string, err error)

	// ReplyMessage replies to an existing message.
	ReplyMessage(ctx context.Context, replyToID, msgType, content string) (messageID string, err error)

	// UpdateMessage updates an existing message in-place.
	UpdateMessage(ctx context.Context, messageID, msgType, content string) error

	// AddReaction adds an emoji reaction to a message.
	AddReaction(ctx context.Context, messageID, emojiType string) error

	// UploadImage uploads an image and returns its key.
	UploadImage(ctx context.Context, payload []byte) (imageKey string, err error)

	// UploadFile uploads a file and returns its key.
	UploadFile(ctx context.Context, payload []byte, fileName, fileType string) (fileKey string, err error)

	// ListMessages retrieves recent messages from a chat.
	ListMessages(ctx context.Context, chatID string, pageSize int) ([]*larkim.Message, error)
}
