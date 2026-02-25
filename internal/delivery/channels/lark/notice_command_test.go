package lark

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"

	lru "github.com/hashicorp/golang-lru/v2"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestIsNoticeCommand(t *testing.T) {
	g := &Gateway{}
	tests := []struct {
		input string
		want  bool
	}{
		{input: "/notice", want: true},
		{input: "/notice status", want: true},
		{input: "/Notice off", want: true},
		{input: "/notices", want: false},
		{input: "notice", want: false},
	}

	for _, tt := range tests {
		if got := g.isNoticeCommand(tt.input); got != tt.want {
			t.Fatalf("isNoticeCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestHandleNoticeCommandBindStatusOff(t *testing.T) {
	tmp := t.TempDir()
	now := time.Date(2026, 2, 9, 9, 0, 0, 0, time.UTC)
	store := &noticeStateStore{
		path:   filepath.Join(tmp, "lark-notice.state.json"),
		logger: logging.OrNop(nil),
		now:    func() time.Time { return now },
	}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg:         Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowGroups: true}, AppID: "test", AppSecret: "secret"},
		logger:      logging.OrNop(nil),
		messenger:   recorder,
		noticeState: store,
	}

	bindMsg := &incomingMessage{
		chatID:    "oc_notice_chat",
		messageID: "om_notice_bind",
		senderID:  "ou_notice_setter",
		content:   "/notice",
		isGroup:   true,
	}
	gw.handleNoticeCommand(bindMsg)

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) != 1 {
		t.Fatalf("expected one bind reply, got %#v", calls)
	}
	bindText := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(bindText, "已将当前群设置为通知群") {
		t.Fatalf("unexpected bind reply: %q", bindText)
	}
	if !strings.Contains(bindText, "oc_notice_chat") {
		t.Fatalf("expected chat id in bind reply, got %q", bindText)
	}

	statusMsg := &incomingMessage{
		chatID:    "oc_notice_chat",
		messageID: "om_notice_status",
		senderID:  "ou_notice_setter",
		content:   "/notice status",
		isGroup:   true,
	}
	gw.handleNoticeCommand(statusMsg)

	calls = recorder.CallsByMethod("ReplyMessage")
	if len(calls) != 2 {
		t.Fatalf("expected two replies after status, got %#v", calls)
	}
	statusText := extractTextContent(calls[1].Content, nil)
	if !strings.Contains(statusText, "当前通知群") {
		t.Fatalf("unexpected status reply: %q", statusText)
	}
	if !strings.Contains(statusText, "oc_notice_chat") {
		t.Fatalf("expected chat id in status reply, got %q", statusText)
	}

	offMsg := &incomingMessage{
		chatID:    "oc_notice_chat",
		messageID: "om_notice_off",
		senderID:  "ou_notice_setter",
		content:   "/notice off",
		isGroup:   true,
	}
	gw.handleNoticeCommand(offMsg)

	calls = recorder.CallsByMethod("ReplyMessage")
	if len(calls) != 3 {
		t.Fatalf("expected three replies after off, got %#v", calls)
	}
	offText := extractTextContent(calls[2].Content, nil)
	if !strings.Contains(offText, "已清除通知群绑定") {
		t.Fatalf("unexpected off reply: %q", offText)
	}
	_, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load() after off failed: %v", err)
	}
	if ok {
		t.Fatal("expected notice binding cleared")
	}
}

func TestHandleNoticeCommandRequireGroup(t *testing.T) {
	tmp := t.TempDir()
	store := &noticeStateStore{
		path:   filepath.Join(tmp, "lark-notice.state.json"),
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now().UTC() },
	}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg:         Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
		logger:      logging.OrNop(nil),
		messenger:   recorder,
		noticeState: store,
	}
	msg := &incomingMessage{
		chatID:    "oc_dm_chat",
		messageID: "om_notice_dm",
		senderID:  "ou_notice_setter",
		content:   "/notice",
		isGroup:   false,
	}
	gw.handleNoticeCommand(msg)
	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) != 1 {
		t.Fatalf("expected one dm reply, got %#v", calls)
	}
	txt := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(txt, "请在目标群里发送 /notice") {
		t.Fatalf("unexpected dm reply: %q", txt)
	}
}

func TestHandleMessageNoticeCommandBypassesInFlightInjection(t *testing.T) {
	openID := "ou_sender_notice_running"
	chatID := "oc_chat_notice_running"
	msgType := "text"
	chatType := "group"
	msgID1 := "om_msg_notice_running_1"
	msgID2 := "om_msg_notice_running_2"
	content1 := `{"text":"run something"}`
	content2 := `{"text":"/notice"}`

	tmp := t.TempDir()
	store := &noticeStateStore{
		path:   filepath.Join(tmp, "lark-notice.state.json"),
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now().UTC() },
	}
	executor := &blockingExecutor{
		started: make(chan struct{}),
		finish:  make(chan struct{}),
	}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg:         Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowGroups: true}, AppID: "test", AppSecret: "secret"},
		agent:       executor,
		logger:      logging.OrNop(nil),
		messenger:   recorder,
		noticeState: store,
		now:         func() time.Time { return time.Now() },
	}
	cache, _ := lru.New[string, time.Time](16)
	gw.dedupCache = cache

	event1 := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID1,
				Content:     &content1,
			},
			Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &openID}},
		},
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := gw.handleMessage(context.Background(), event1); err != nil {
			t.Errorf("handleMessage failed: %v", err)
		}
	}()
	<-executor.started

	event2 := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID2,
				Content:     &content2,
			},
			Sender: &larkim.EventSender{SenderId: &larkim.UserId{OpenId: &openID}},
		},
	}
	if err := gw.handleMessage(context.Background(), event2); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	executor.mu.Lock()
	callCount := executor.callCount
	executor.mu.Unlock()
	if callCount != 1 {
		t.Fatalf("expected /notice not to trigger/inject new task call, got %d", callCount)
	}

	binding, ok, err := store.Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !ok {
		t.Fatal("expected notice binding to be saved")
	}
	if binding.ChatID != chatID {
		t.Fatalf("saved chat_id = %q, want %q", binding.ChatID, chatID)
	}

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		calls = recorder.CallsByMethod("SendMessage")
	}
	if len(calls) == 0 {
		t.Fatal("expected a /notice reply")
	}
	if txt := extractTextContent(calls[len(calls)-1].Content, nil); !strings.Contains(txt, "已将当前群设置为通知群") {
		t.Fatalf("unexpected /notice reply: %q", txt)
	}

	close(executor.finish)
	wg.Wait()
	gw.WaitForTasks()
}

var _ agent.EventListener = agent.NoopEventListener{}
