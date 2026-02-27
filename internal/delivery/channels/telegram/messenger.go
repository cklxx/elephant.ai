package telegram

import (
	"context"
	"fmt"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Messenger abstracts Telegram message send/edit operations.
type Messenger interface {
	SendText(ctx context.Context, chatID int64, text string, replyToMsgID int) (int, error)
	EditText(ctx context.Context, chatID int64, messageID int, text string) error
}

// sdkMessenger implements Messenger via the telego Bot API.
type sdkMessenger struct {
	bot *telego.Bot
}

func newSDKMessenger(bot *telego.Bot) *sdkMessenger {
	return &sdkMessenger{bot: bot}
}

func (m *sdkMessenger) SendText(ctx context.Context, chatID int64, text string, replyToMsgID int) (int, error) {
	chunks := splitForTelegram(text, telegramMaxMessageLen)
	var lastMsgID int
	for _, chunk := range chunks {
		params := tu.Message(tu.ID(chatID), chunk)
		if replyToMsgID > 0 {
			params = params.WithReplyParameters(&telego.ReplyParameters{MessageID: replyToMsgID})
		}
		msg, err := m.bot.SendMessage(ctx, params)
		if err != nil {
			return lastMsgID, fmt.Errorf("send message: %w", err)
		}
		lastMsgID = msg.MessageID
		// Only reply-thread the first chunk.
		replyToMsgID = 0
	}
	return lastMsgID, nil
}

func (m *sdkMessenger) EditText(ctx context.Context, chatID int64, messageID int, text string) error {
	truncated := truncateWithEllipsis(text, telegramMaxMessageLen)
	params := &telego.EditMessageTextParams{
		ChatID:    tu.ID(chatID),
		MessageID: messageID,
		Text:      truncated,
	}
	_, err := m.bot.EditMessageText(ctx, params)
	if err != nil {
		return fmt.Errorf("edit message: %w", err)
	}
	return nil
}
