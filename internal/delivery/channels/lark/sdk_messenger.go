package lark

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// sdkMessenger implements LarkMessenger using the real Lark SDK client.
type sdkMessenger struct {
	client *lark.Client
}

// newSDKMessenger wraps a Lark SDK client as a LarkMessenger.
func newSDKMessenger(client *lark.Client) *sdkMessenger {
	return &sdkMessenger{client: client}
}

func (m *sdkMessenger) SendMessage(ctx context.Context, chatID, msgType, content string) (string, error) {
	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()
	resp, err := m.client.Im.Message.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark send message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", nil
	}
	return *resp.Data.MessageId, nil
}

func (m *sdkMessenger) ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error) {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(replyToID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			Build()).
		Build()
	resp, err := m.client.Im.Message.Reply(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark reply message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.MessageId == nil {
		return "", nil
	}
	return *resp.Data.MessageId, nil
}

func (m *sdkMessenger) UpdateMessage(ctx context.Context, messageID, msgType, content string) error {
	req := larkim.NewUpdateMessageReqBuilder().
		MessageId(messageID).
		Body(larkim.NewUpdateMessageReqBodyBuilder().
			MsgType(msgType).
			Content(content).
			Build()).
		Build()
	resp, err := m.client.Im.Message.Update(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("lark update message error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (m *sdkMessenger) AddReaction(ctx context.Context, messageID, emojiType string) error {
	req := larkim.NewCreateMessageReactionReqBuilder().
		MessageId(messageID).
		Body(larkim.NewCreateMessageReactionReqBodyBuilder().
			ReactionType(larkim.NewEmojiBuilder().
				EmojiType(emojiType).
				Build()).
			Build()).
		Build()
	resp, err := m.client.Im.V1.MessageReaction.Create(ctx, req)
	if err != nil {
		return err
	}
	if !resp.Success() {
		return fmt.Errorf("lark add reaction error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

func (m *sdkMessenger) UploadImage(ctx context.Context, payload []byte) (string, error) {
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType("message").
			Image(bytes.NewReader(payload)).
			Build()).
		Build()
	resp, err := m.client.Im.V1.Image.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark image upload failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.ImageKey == nil || strings.TrimSpace(*resp.Data.ImageKey) == "" {
		return "", fmt.Errorf("lark image upload missing image_key")
	}
	return *resp.Data.ImageKey, nil
}

func (m *sdkMessenger) UploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	req := larkim.NewCreateFileReqBuilder().
		Body(larkim.NewCreateFileReqBodyBuilder().
			FileType(fileType).
			FileName(fileName).
			File(bytes.NewReader(payload)).
			Build()).
		Build()
	resp, err := m.client.Im.V1.File.Create(ctx, req)
	if err != nil {
		return "", err
	}
	if !resp.Success() {
		return "", fmt.Errorf("lark file upload failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil || resp.Data.FileKey == nil || strings.TrimSpace(*resp.Data.FileKey) == "" {
		return "", fmt.Errorf("lark file upload missing file_key")
	}
	return *resp.Data.FileKey, nil
}

func (m *sdkMessenger) ListMessages(ctx context.Context, chatID string, pageSize int) ([]*larkim.Message, error) {
	req := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		SortType("ByCreateTimeDesc").
		PageSize(pageSize).
		Build()
	resp, err := m.client.Im.Message.List(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("lark chat history API call failed: %w", err)
	}
	if !resp.Success() {
		return nil, fmt.Errorf("lark chat history API error: code=%d msg=%s", resp.Code, resp.Msg)
	}
	if resp.Data == nil {
		return nil, nil
	}
	return resp.Data.Items, nil
}
