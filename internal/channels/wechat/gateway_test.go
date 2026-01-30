package wechat

import (
	"strings"
	"testing"

	"alex/internal/channels"

	"github.com/eatmoreapple/openwechat"
)

func TestConversationKeyPrefersID(t *testing.T) {
	gateway := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "wechat"}}}
	user := &openwechat.User{Uin: 123, UserName: "wxid_abc", NickName: "nick"}
	got := gateway.conversationKey(user)
	if got != "123" {
		t.Fatalf("expected ID-based key, got %q", got)
	}
}

func TestConversationKeyFallbackToUserName(t *testing.T) {
	gateway := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "wechat"}}}
	user := &openwechat.User{UserName: "wxid_abc", NickName: "nick"}
	got := gateway.conversationKey(user)
	if got != "wxid_abc" {
		t.Fatalf("expected username-based key, got %q", got)
	}
}

func TestSessionIDForConversationStable(t *testing.T) {
	gateway := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "wechat"}}}
	first := gateway.sessionIDForConversation("conv-1")
	second := gateway.sessionIDForConversation("conv-1")
	if first != second {
		t.Fatalf("expected deterministic session id, got %q vs %q", first, second)
	}
	if !strings.HasPrefix(first, "wechat-") {
		t.Fatalf("expected prefix 'wechat-', got %q", first)
	}
}

func TestMentionHelpers(t *testing.T) {
	gateway := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "wechat"}}}
	gateway.self = &openwechat.Self{User: &openwechat.User{NickName: "robot"}}

	content := "@robot hello"
	if !gateway.isMentioned(content) {
		t.Fatalf("expected mention detection for %q", content)
	}
	trimmed := gateway.stripMention(content)
	if trimmed != "hello" {
		t.Fatalf("expected mention stripped, got %q", trimmed)
	}

	if gateway.isMentioned("hello") {
		t.Fatalf("expected no mention for plain content")
	}
}
