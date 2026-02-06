package lark

import (
	"context"
	"fmt"
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestRecordingMessengerSendMessage(t *testing.T) {
	rec := NewRecordingMessenger()
	id, err := rec.SendMessage(context.Background(), "chat_1", "text", `{"text":"hi"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "om_recorded_1" {
		t.Fatalf("expected auto-generated ID, got %q", id)
	}

	calls := rec.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Method != "SendMessage" || calls[0].ChatID != "chat_1" {
		t.Fatalf("unexpected call: %+v", calls[0])
	}
}

func TestRecordingMessengerReplyMessage(t *testing.T) {
	rec := NewRecordingMessenger()
	rec.NextMessageID = "om_custom"
	id, err := rec.ReplyMessage(context.Background(), "om_parent", "text", `{"text":"reply"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "om_custom" {
		t.Fatalf("expected custom ID, got %q", id)
	}

	calls := rec.CallsByMethod("ReplyMessage")
	if len(calls) != 1 {
		t.Fatalf("expected 1 ReplyMessage call, got %d", len(calls))
	}
	if calls[0].ReplyTo != "om_parent" {
		t.Fatalf("expected reply to om_parent, got %q", calls[0].ReplyTo)
	}
}

func TestRecordingMessengerUpdateMessage(t *testing.T) {
	rec := NewRecordingMessenger()
	err := rec.UpdateMessage(context.Background(), "om_msg_1", "text", `{"text":"updated"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := rec.CallsByMethod("UpdateMessage")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].MsgID != "om_msg_1" {
		t.Fatalf("expected message ID om_msg_1, got %q", calls[0].MsgID)
	}
}

func TestRecordingMessengerAddReaction(t *testing.T) {
	rec := NewRecordingMessenger()
	err := rec.AddReaction(context.Background(), "om_msg_1", "SMILE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := rec.CallsByMethod("AddReaction")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Emoji != "SMILE" {
		t.Fatalf("expected SMILE emoji, got %q", calls[0].Emoji)
	}
}

func TestRecordingMessengerUploadImage(t *testing.T) {
	rec := NewRecordingMessenger()
	rec.NextImageKey = "img_test_123"
	key, err := rec.UploadImage(context.Background(), []byte("png-data"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "img_test_123" {
		t.Fatalf("expected img_test_123, got %q", key)
	}

	calls := rec.CallsByMethod("UploadImage")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if string(calls[0].Payload) != "png-data" {
		t.Fatalf("expected payload 'png-data', got %q", string(calls[0].Payload))
	}
}

func TestRecordingMessengerUploadFile(t *testing.T) {
	rec := NewRecordingMessenger()
	key, err := rec.UploadFile(context.Background(), []byte("pdf-data"), "report.pdf", "pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "file_recorded" {
		t.Fatalf("expected default file key, got %q", key)
	}

	calls := rec.CallsByMethod("UploadFile")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].FileName != "report.pdf" {
		t.Fatalf("expected filename report.pdf, got %q", calls[0].FileName)
	}
}

func TestRecordingMessengerListMessages(t *testing.T) {
	ts := "1706500000000"
	stype := "user"
	sid := "ou_1"
	mtype := "text"
	body := `{"text":"test"}`
	rec := NewRecordingMessenger()
	rec.ListMessagesResult = []*larkim.Message{
		{
			CreateTime: &ts,
			Sender:     &larkim.Sender{SenderType: &stype, Id: &sid},
			MsgType:    &mtype,
			Body:       &larkim.MessageBody{Content: &body},
		},
	}

	items, err := rec.ListMessages(context.Background(), "chat_1", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}

	calls := rec.CallsByMethod("ListMessages")
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].PageSize != 10 {
		t.Fatalf("expected page size 10, got %d", calls[0].PageSize)
	}
}

func TestRecordingMessengerNextError(t *testing.T) {
	rec := NewRecordingMessenger()
	rec.NextError = fmt.Errorf("test error")

	_, err := rec.SendMessage(context.Background(), "chat_1", "text", `{}`)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "test error" {
		t.Fatalf("expected 'test error', got %q", err.Error())
	}

	// NextError should be cleared after first use.
	_, err = rec.SendMessage(context.Background(), "chat_1", "text", `{}`)
	if err != nil {
		t.Fatalf("expected nil error after clearing, got %v", err)
	}
}

func TestRecordingMessengerReset(t *testing.T) {
	rec := NewRecordingMessenger()
	_, _ = rec.SendMessage(context.Background(), "chat_1", "text", `{}`)
	if len(rec.Calls()) != 1 {
		t.Fatal("expected 1 call before reset")
	}

	rec.Reset()
	if len(rec.Calls()) != 0 {
		t.Fatal("expected 0 calls after reset")
	}

	// IDs should restart.
	id, _ := rec.SendMessage(context.Background(), "chat_1", "text", `{}`)
	if id != "om_recorded_1" {
		t.Fatalf("expected ID counter to restart, got %q", id)
	}
}

func TestRecordingMessengerAutoIncrementIDs(t *testing.T) {
	rec := NewRecordingMessenger()
	id1, _ := rec.SendMessage(context.Background(), "c", "text", `{}`)
	id2, _ := rec.ReplyMessage(context.Background(), "r", "text", `{}`)
	id3, _ := rec.SendMessage(context.Background(), "c", "text", `{}`)

	if id1 != "om_recorded_1" || id2 != "om_recorded_2" || id3 != "om_recorded_3" {
		t.Fatalf("expected sequential IDs, got %q %q %q", id1, id2, id3)
	}
}
