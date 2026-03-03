package lark

import (
	"context"
	"fmt"
	"sync"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// Messenger method name constants used in MessengerCall.Method.
const (
	MethodSendMessage    = "SendMessage"
	MethodReplyMessage   = "ReplyMessage"
	MethodUpdateMessage  = "UpdateMessage"
	MethodAddReaction    = "AddReaction"
	MethodDeleteReaction = "DeleteReaction"
	MethodUploadImage    = "UploadImage"
	MethodUploadFile     = "UploadFile"
	MethodListMessages   = "ListMessages"
)

// MessengerCall records a single outbound call made through a LarkMessenger.
type MessengerCall struct {
	Method     string // one of the Method* constants above
	ChatID     string
	MsgType    string
	Content    string
	ReplyTo    string
	MsgID      string
	Emoji      string
	ReactionID string
	FileName   string
	FileType   string
	PageSize   int
	Payload    []byte
}

// RecordingMessenger implements LarkMessenger by recording all outbound calls
// for later assertion in tests. It returns configurable responses and errors.
type RecordingMessenger struct {
	mu    sync.Mutex
	calls []MessengerCall

	// NextMessageID is returned as the messageID for Send/Reply operations.
	// If empty, a sequential "om_recorded_N" ID is generated.
	NextMessageID string

	// NextImageKey is returned by UploadImage. Defaults to "img_recorded".
	NextImageKey string

	// NextFileKey is returned by UploadFile. Defaults to "file_recorded".
	NextFileKey string

	// NextError, when set, is returned by the next call (any method) and then cleared.
	NextError error

	// ListMessagesResult is returned by ListMessages.
	ListMessagesResult []*larkim.Message

	sendCount int
}

// NewRecordingMessenger creates a RecordingMessenger with sensible defaults.
func NewRecordingMessenger() *RecordingMessenger {
	return &RecordingMessenger{}
}

func (r *RecordingMessenger) record(call MessengerCall) {
	r.calls = append(r.calls, call)
}

func (r *RecordingMessenger) popError() error {
	if r.NextError != nil {
		err := r.NextError
		r.NextError = nil
		return err
	}
	return nil
}

func (r *RecordingMessenger) nextMsgID() string {
	if r.NextMessageID != "" {
		id := r.NextMessageID
		r.NextMessageID = ""
		return id
	}
	r.sendCount++
	return fmt.Sprintf("om_recorded_%d", r.sendCount)
}

func (r *RecordingMessenger) SendMessage(_ context.Context, chatID, msgType, content string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodSendMessage, ChatID: chatID, MsgType: msgType, Content: content})
	if err := r.popError(); err != nil {
		return "", err
	}
	return r.nextMsgID(), nil
}

func (r *RecordingMessenger) ReplyMessage(_ context.Context, replyToID, msgType, content string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodReplyMessage, ReplyTo: replyToID, MsgType: msgType, Content: content})
	if err := r.popError(); err != nil {
		return "", err
	}
	return r.nextMsgID(), nil
}

func (r *RecordingMessenger) UpdateMessage(_ context.Context, messageID, msgType, content string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodUpdateMessage, MsgID: messageID, MsgType: msgType, Content: content})
	return r.popError()
}

func (r *RecordingMessenger) AddReaction(_ context.Context, messageID, emojiType string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodAddReaction, MsgID: messageID, Emoji: emojiType})
	if err := r.popError(); err != nil {
		return "", err
	}
	return fmt.Sprintf("reaction_%s_%s", messageID, emojiType), nil
}

func (r *RecordingMessenger) DeleteReaction(_ context.Context, messageID, reactionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodDeleteReaction, MsgID: messageID, ReactionID: reactionID})
	return r.popError()
}

func (r *RecordingMessenger) UploadImage(_ context.Context, payload []byte) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodUploadImage, Payload: payload})
	if err := r.popError(); err != nil {
		return "", err
	}
	key := r.NextImageKey
	if key == "" {
		key = "img_recorded"
	}
	r.NextImageKey = ""
	return key, nil
}

func (r *RecordingMessenger) UploadFile(_ context.Context, payload []byte, fileName, fileType string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodUploadFile, Payload: payload, FileName: fileName, FileType: fileType})
	if err := r.popError(); err != nil {
		return "", err
	}
	key := r.NextFileKey
	if key == "" {
		key = "file_recorded"
	}
	r.NextFileKey = ""
	return key, nil
}

func (r *RecordingMessenger) ListMessages(_ context.Context, chatID string, pageSize int) ([]*larkim.Message, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.record(MessengerCall{Method: MethodListMessages, ChatID: chatID, PageSize: pageSize})
	if err := r.popError(); err != nil {
		return nil, err
	}
	return r.ListMessagesResult, nil
}

// Calls returns a snapshot of all recorded calls.
func (r *RecordingMessenger) Calls() []MessengerCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]MessengerCall, len(r.calls))
	copy(out, r.calls)
	return out
}

// CallsByMethod returns calls filtered by method name.
func (r *RecordingMessenger) CallsByMethod(method string) []MessengerCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []MessengerCall
	for _, c := range r.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// Reset clears all recorded calls.
func (r *RecordingMessenger) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = nil
	r.sendCount = 0
}
