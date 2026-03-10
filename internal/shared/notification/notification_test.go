package notification

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChannelConstants(t *testing.T) {
	assert.Equal(t, "lark", ChannelLark)
	assert.Equal(t, "moltbook", ChannelMoltbook)
	assert.NotEqual(t, ChannelLark, ChannelMoltbook)
}

func TestTargetFields(t *testing.T) {
	target := Target{Channel: ChannelLark, ChatID: "chat-123"}
	assert.Equal(t, ChannelLark, target.Channel)
	assert.Equal(t, "chat-123", target.ChatID)
}

func TestTargetZeroValue(t *testing.T) {
	var target Target
	assert.Empty(t, target.Channel)
	assert.Empty(t, target.ChatID)
}

// mockNotifier implements Notifier for verifying interface conformance.
type mockNotifier struct {
	lastCtx     context.Context
	lastTarget  Target
	lastContent string
	err         error
}

func (m *mockNotifier) Send(ctx context.Context, target Target, content string) error {
	m.lastCtx = ctx
	m.lastTarget = target
	m.lastContent = content
	return m.err
}

func TestNotifierInterfaceConformance(t *testing.T) {
	var n Notifier = &mockNotifier{}
	err := n.Send(context.Background(), Target{Channel: ChannelLark, ChatID: "c1"}, "hello")
	assert.NoError(t, err)
}

func TestNotifierReturnsError(t *testing.T) {
	n := &mockNotifier{err: errors.New("send failed")}
	err := n.Send(context.Background(), Target{}, "msg")
	assert.EqualError(t, err, "send failed")
}

func TestNotifierCapturesArguments(t *testing.T) {
	m := &mockNotifier{}
	ctx := context.WithValue(context.Background(), struct{}{}, "test")
	target := Target{Channel: ChannelMoltbook, ChatID: "chat-456"}
	content := "alert message"

	_ = m.Send(ctx, target, content)

	assert.Equal(t, ctx, m.lastCtx)
	assert.Equal(t, target, m.lastTarget)
	assert.Equal(t, content, m.lastContent)
}
