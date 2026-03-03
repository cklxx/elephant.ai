package shared

import (
	"context"
	"reflect"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/backup"
	tmr "alex/internal/shared/timer"
)

type testApprover struct{}

func (testApprover) RequestApproval(ctx context.Context, request *tools.ApprovalRequest) (*tools.ApprovalResponse, error) {
	return &tools.ApprovalResponse{Approved: true}, nil
}

type testLarkOAuthService struct{}

func (testLarkOAuthService) UserAccessToken(ctx context.Context, openID string) (string, error) {
	return "token", nil
}

func (testLarkOAuthService) StartURL() string {
	return "https://example.com/auth"
}

type testTimerManager struct{}

func (testTimerManager) Add(t *tmr.Timer) error {
	return nil
}

func (testTimerManager) Cancel(timerID string) error {
	return nil
}

func (testTimerManager) List(userID string) []tmr.Timer {
	return nil
}

func (testTimerManager) Get(timerID string) (tmr.Timer, bool) {
	return tmr.Timer{}, false
}

type testEventListener struct{}

func (testEventListener) OnEvent(event agent.AgentEvent) {}

type testLarkMessenger struct{}

func (testLarkMessenger) SendMessage(ctx context.Context, chatID, msgType, content string) (string, error) {
	return "msg-id", nil
}

func (testLarkMessenger) ReplyMessage(ctx context.Context, replyToID, msgType, content string) (string, error) {
	return "reply-id", nil
}

func TestContextGetters_NilContextReturnsZeroValues(t *testing.T) {
	var nilCtx context.Context

	if got := GetApproverFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil approver, got %T", got)
	}
	if got := GetBackupManagerFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil backup manager, got %T", got)
	}
	if got := GetToolSessionIDFromContext(nilCtx); got != "" {
		t.Fatalf("expected empty session id, got %q", got)
	}
	if got := GetAutoApproveFromContext(nilCtx); got {
		t.Fatal("expected false auto approve")
	}
	if got := GetParentListenerFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil parent listener, got %T", got)
	}
	if got := LarkClientFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil lark client, got %T", got)
	}
	if got := LarkMessengerFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil lark messenger, got %T", got)
	}
	if got := LarkChatIDFromContext(nilCtx); got != "" {
		t.Fatalf("expected empty lark chat id, got %q", got)
	}
	if got := LarkMessageIDFromContext(nilCtx); got != "" {
		t.Fatalf("expected empty lark message id, got %q", got)
	}
	if got := LarkOAuthFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil lark oauth service, got %T", got)
	}
	if got := LarkBaseDomainFromContext(nilCtx); got != "" {
		t.Fatalf("expected empty lark base domain, got %q", got)
	}
	if got := LarkTenantCalendarIDFromContext(nilCtx); got != "" {
		t.Fatalf("expected empty lark tenant calendar id, got %q", got)
	}
	if got := TimerManagerFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil timer manager, got %T", got)
	}
	if got := SchedulerFromContext(nilCtx); got != nil {
		t.Fatalf("expected nil scheduler, got %T", got)
	}
	if got := GetAutoUploadConfig(nilCtx); !reflect.DeepEqual(got, AutoUploadConfig{}) {
		t.Fatalf("expected zero auto upload config, got %#v", got)
	}
}

func TestContextGetters_WrongTypesReturnZeroValues(t *testing.T) {
	ctx := context.Background()

	if got := GetApproverFromContext(context.WithValue(ctx, ApproverKey, "wrong")); got != nil {
		t.Fatalf("expected nil approver, got %T", got)
	}
	if got := GetBackupManagerFromContext(context.WithValue(ctx, BackupManagerKey, "wrong")); got != nil {
		t.Fatalf("expected nil backup manager, got %T", got)
	}
	if got := GetToolSessionIDFromContext(context.WithValue(ctx, ToolSessionIDKey, 123)); got != "" {
		t.Fatalf("expected empty session id, got %q", got)
	}
	if got := GetAutoApproveFromContext(context.WithValue(ctx, AutoApproveKey, "wrong")); got {
		t.Fatal("expected false auto approve")
	}
	if got := GetParentListenerFromContext(context.WithValue(ctx, parentListenerKey{}, "wrong")); got != nil {
		t.Fatalf("expected nil parent listener, got %T", got)
	}
	if got := LarkMessengerFromContext(context.WithValue(ctx, larkMessengerKey, "wrong")); got != nil {
		t.Fatalf("expected nil lark messenger, got %T", got)
	}
	if got := LarkChatIDFromContext(context.WithValue(ctx, larkChatIDKey, 123)); got != "" {
		t.Fatalf("expected empty lark chat id, got %q", got)
	}
	if got := LarkMessageIDFromContext(context.WithValue(ctx, larkMessageIDKey, 123)); got != "" {
		t.Fatalf("expected empty lark message id, got %q", got)
	}
	if got := LarkOAuthFromContext(context.WithValue(ctx, larkOAuthKey, "wrong")); got != nil {
		t.Fatalf("expected nil lark oauth service, got %T", got)
	}
	if got := LarkBaseDomainFromContext(context.WithValue(ctx, larkBaseDomainKey, 123)); got != "" {
		t.Fatalf("expected empty lark base domain, got %q", got)
	}
	if got := LarkTenantCalendarIDFromContext(context.WithValue(ctx, larkTenantCalKey, 123)); got != "" {
		t.Fatalf("expected empty lark tenant calendar id, got %q", got)
	}
	if got := TimerManagerFromContext(context.WithValue(ctx, timerManagerKey, "wrong")); got != nil {
		t.Fatalf("expected nil timer manager, got %T", got)
	}
	if got := GetAutoUploadConfig(context.WithValue(ctx, autoUploadKey, "wrong")); !reflect.DeepEqual(got, AutoUploadConfig{}) {
		t.Fatalf("expected zero auto upload config, got %#v", got)
	}
}

func TestContextGetters_RoundTrip(t *testing.T) {
	approver := &testApprover{}
	backupManager := &backup.Manager{}
	listener := &testEventListener{}
	larkClient := &struct{ Name string }{Name: "client"}
	larkMessenger := &testLarkMessenger{}
	oauth := &testLarkOAuthService{}
	timerManager := &testTimerManager{}
	scheduler := &struct{ Name string }{Name: "scheduler"}
	autoUploadCfg := AutoUploadConfig{
		Enabled:   true,
		MaxBytes:  1024,
		AllowExts: []string{".md", ".txt"},
	}

	ctx := context.Background()
	ctx = WithApprover(ctx, approver)
	ctx = WithBackupManager(ctx, backupManager)
	ctx = WithToolSessionID(ctx, "session-1")
	ctx = WithAutoApprove(ctx, true)
	ctx = WithParentListener(ctx, listener)
	ctx = WithLarkClient(ctx, larkClient)
	ctx = WithLarkMessenger(ctx, larkMessenger)
	ctx = WithLarkChatID(ctx, "chat-1")
	ctx = WithLarkMessageID(ctx, "msg-1")
	ctx = WithLarkOAuth(ctx, oauth)
	ctx = WithLarkBaseDomain(ctx, "https://open.feishu.cn")
	ctx = WithLarkTenantCalendarID(ctx, "calendar-1")
	ctx = WithTimerManager(ctx, timerManager)
	ctx = WithScheduler(ctx, scheduler)
	ctx = WithAutoUploadConfig(ctx, autoUploadCfg)

	if got := GetApproverFromContext(ctx); got != approver {
		t.Fatalf("unexpected approver: %#v", got)
	}
	if got := GetBackupManagerFromContext(ctx); got != backupManager {
		t.Fatalf("unexpected backup manager: %#v", got)
	}
	if got := GetToolSessionIDFromContext(ctx); got != "session-1" {
		t.Fatalf("unexpected session id: %q", got)
	}
	if got := GetAutoApproveFromContext(ctx); !got {
		t.Fatal("expected true auto approve")
	}
	if got := GetParentListenerFromContext(ctx); got != listener {
		t.Fatalf("unexpected parent listener: %#v", got)
	}
	if got := LarkClientFromContext(ctx); got != larkClient {
		t.Fatalf("unexpected lark client: %#v", got)
	}
	if got := LarkMessengerFromContext(ctx); got != larkMessenger {
		t.Fatalf("unexpected lark messenger: %#v", got)
	}
	if got := LarkChatIDFromContext(ctx); got != "chat-1" {
		t.Fatalf("unexpected lark chat id: %q", got)
	}
	if got := LarkMessageIDFromContext(ctx); got != "msg-1" {
		t.Fatalf("unexpected lark message id: %q", got)
	}
	if got := LarkOAuthFromContext(ctx); got != oauth {
		t.Fatalf("unexpected lark oauth service: %#v", got)
	}
	if got := LarkBaseDomainFromContext(ctx); got != "https://open.feishu.cn" {
		t.Fatalf("unexpected lark base domain: %q", got)
	}
	if got := LarkTenantCalendarIDFromContext(ctx); got != "calendar-1" {
		t.Fatalf("unexpected lark tenant calendar id: %q", got)
	}
	if got := TimerManagerFromContext(ctx); got != timerManager {
		t.Fatalf("unexpected timer manager: %#v", got)
	}
	if got := SchedulerFromContext(ctx); got != scheduler {
		t.Fatalf("unexpected scheduler: %#v", got)
	}
	if got := GetAutoUploadConfig(ctx); !reflect.DeepEqual(got, autoUploadCfg) {
		t.Fatalf("unexpected auto upload config: %#v", got)
	}
}

func TestContextValueOr_ReturnsFallbackWhenValueMissing(t *testing.T) {
	got := contextValueOr[string](context.Background(), ToolSessionIDKey, "fallback")
	if got != "fallback" {
		t.Fatalf("expected fallback value, got %q", got)
	}
}
