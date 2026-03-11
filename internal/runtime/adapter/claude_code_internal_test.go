package adapter

import (
	"context"
	"errors"
	"testing"
	"time"

	"alex/internal/runtime/panel"
)

type internalMockPane struct {
	captureText string
	captureErr  error
}

func (p *internalMockPane) PaneID() int { return 1 }

func (p *internalMockPane) InjectText(context.Context, string) error { return nil }

func (p *internalMockPane) Submit(context.Context) error { return nil }

func (p *internalMockPane) Send(context.Context, string) error { return nil }

func (p *internalMockPane) SendKey(context.Context, string) error { return nil }

func (p *internalMockPane) CaptureOutput(context.Context) (string, error) {
	return p.captureText, p.captureErr
}

func (p *internalMockPane) Activate(context.Context) error { return nil }

func (p *internalMockPane) Kill(context.Context) error { return nil }

var _ panel.PaneIface = (*internalMockPane)(nil)

type internalHookSink struct {
	completed []string
	failed    []string
}

func (s *internalHookSink) OnHeartbeat(string) {}

func (s *internalHookSink) OnCompleted(sessionID, _ string) {
	s.completed = append(s.completed, sessionID)
}

func (s *internalHookSink) OnFailed(sessionID, errMsg string) {
	s.failed = append(s.failed, sessionID+":"+errMsg)
}

func (s *internalHookSink) OnNeedsInput(string, string) {}

func TestClaudeCodeWatchForCompletion_CaptureError(t *testing.T) {
	t.Parallel()

	pane := &internalMockPane{captureErr: errors.New("pane closed")}
	sink := &internalHookSink{}
	cc := &ClaudeCodeAdapter{
		sink:  sink,
		panes: map[string]panel.PaneIface{"sess-1": pane},
	}

	done := make(chan struct{})
	go func() {
		cc.watchForCompletion(context.Background(), "sess-1", pane)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(ccPollInterval + time.Second):
		t.Fatal("watchForCompletion did not exit")
	}

	if len(sink.failed) != 1 || sink.failed[0] != "sess-1:pane closed unexpectedly" {
		t.Fatalf("failed callbacks = %v, want pane closed failure", sink.failed)
	}
	if got := cc.getPane("sess-1"); got != nil {
		t.Fatalf("pane should be removed after failure, got %T", got)
	}
}

func TestClaudeCodeWatchForCompletion_ShellPrompt(t *testing.T) {
	t.Parallel()

	pane := &internalMockPane{captureText: "user@host:~$ "}
	sink := &internalHookSink{}
	cc := &ClaudeCodeAdapter{
		sink:  sink,
		panes: map[string]panel.PaneIface{"sess-2": pane},
	}

	done := make(chan struct{})
	go func() {
		cc.watchForCompletion(context.Background(), "sess-2", pane)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(ccPollInterval + time.Second):
		t.Fatal("watchForCompletion did not exit")
	}

	if len(sink.completed) != 1 || sink.completed[0] != "sess-2" {
		t.Fatalf("completed callbacks = %v, want sess-2", sink.completed)
	}
	if got := cc.getPane("sess-2"); got != nil {
		t.Fatalf("pane should be removed after completion, got %T", got)
	}
}
