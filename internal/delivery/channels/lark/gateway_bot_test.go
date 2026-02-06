package lark

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestIsBotSender(t *testing.T) {
	tests := []struct {
		name     string
		senderType string
		want     bool
	}{
		{"app sender", "app", true},
		{"user sender", "user", false},
		{"empty sender", "", false},
		{"other sender", "other", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &larkim.P2MessageReceiveV1{
				Event: &larkim.P2MessageReceiveV1Data{
					Sender: &larkim.EventSender{
						SenderType: &tt.senderType,
					},
				},
			}
			got := isBotSender(event)
			if got != tt.want {
				t.Fatalf("isBotSender() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBotSender_NilEvent(t *testing.T) {
	if isBotSender(nil) {
		t.Fatal("isBotSender(nil) should return false")
	}

	if isBotSender(&larkim.P2MessageReceiveV1{}) {
		t.Fatal("isBotSender(empty event) should return false")
	}

	if isBotSender(&larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{}}) {
		t.Fatal("isBotSender(event with nil sender) should return false")
	}
}
