package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/app/di"
	larkgw "alex/internal/delivery/channels/lark"
)

type fakeKernelNoticeGateway struct {
	loader func() (string, bool, error)
	sent   []struct {
		chatID string
		text   string
	}
}

func (f *fakeKernelNoticeGateway) NoticeLoader() func() (string, bool, error) {
	return f.loader
}

func (f *fakeKernelNoticeGateway) SendNotification(_ context.Context, chatID, text string) error {
	f.sent = append(f.sent, struct {
		chatID string
		text   string
	}{chatID: chatID, text: text})
	return nil
}

func (f *fakeKernelNoticeGateway) InjectMessageSync(_ context.Context, _ larkgw.InjectSyncRequest) *larkgw.InjectSyncResponse {
	return nil
}

func TestResolveKernelNoticePipelinePrefersGateway(t *testing.T) {
	gw := &fakeKernelNoticeGateway{
		loader: func() (string, bool, error) {
			return "oc_gateway_notice", true, nil
		},
	}
	f := &Foundation{
		Container: &di.Container{LarkGateway: gw},
		Config: Config{
			Channels: ChannelsConfig{
				Lark: LarkGatewayConfig{
					Enabled:   true,
					AppID:     "fallback-id",
					AppSecret: "fallback-secret",
				},
			},
		},
	}

	loader, sender := resolveKernelNoticePipeline(f, nil)
	if loader == nil || sender == nil {
		t.Fatalf("resolveKernelNoticePipeline() returned nil loader/sender: loader=%v sender=%v", loader == nil, sender == nil)
	}

	chatID, ok, err := loader()
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !ok || chatID != "oc_gateway_notice" {
		t.Fatalf("loader() = (%q, %t), want (%q, true)", chatID, ok, "oc_gateway_notice")
	}

	if err := sender(context.Background(), "oc_gateway_notice", "hello from kernel"); err != nil {
		t.Fatalf("sender() error = %v", err)
	}
	if len(gw.sent) != 1 {
		t.Fatalf("gateway send count = %d, want 1", len(gw.sent))
	}
	if gw.sent[0].chatID != "oc_gateway_notice" {
		t.Fatalf("gateway send chat_id = %q, want %q", gw.sent[0].chatID, "oc_gateway_notice")
	}
}

func TestResolveKernelNoticePipelineFallbackDisabledWithoutCredentials(t *testing.T) {
	f := &Foundation{
		Container: &di.Container{},
		Config: Config{
			Channels: ChannelsConfig{
				Lark: LarkGatewayConfig{
					Enabled: true,
				},
			},
		},
	}

	loader, sender := resolveKernelNoticePipeline(f, nil)
	if loader != nil || sender != nil {
		t.Fatalf("resolveKernelNoticePipeline() = (%v, %v), want (nil, nil)", loader != nil, sender != nil)
	}
}

func TestResolveKernelNoticePipelineFallsBackToNoticeStateFile(t *testing.T) {
	tmp := t.TempDir()
	statePath := filepath.Join(tmp, "lark-notice.state.json")
	t.Setenv("LARK_NOTICE_STATE_FILE", statePath)

	payload := `{"chat_id":"oc_file_notice","set_at":"2026-02-25T10:02:11Z","updated_at":"2026-02-25T10:02:11Z"}`
	if err := os.WriteFile(statePath, []byte(payload), 0o644); err != nil {
		t.Fatalf("write notice state: %v", err)
	}

	f := &Foundation{
		Container: &di.Container{},
		Config: Config{
			Channels: ChannelsConfig{
				Lark: LarkGatewayConfig{
					Enabled:    true,
					AppID:      "cli_test",
					AppSecret:  "secret",
					BaseDomain: "https://open.feishu.cn",
				},
			},
		},
	}

	loader, sender := resolveKernelNoticePipeline(f, nil)
	if loader == nil || sender == nil {
		t.Fatalf("resolveKernelNoticePipeline() returned nil loader/sender: loader=%v sender=%v", loader == nil, sender == nil)
	}

	chatID, ok, err := loader()
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !ok || chatID != "oc_file_notice" {
		t.Fatalf("loader() = (%q, %t), want (%q, true)", chatID, ok, "oc_file_notice")
	}
}
