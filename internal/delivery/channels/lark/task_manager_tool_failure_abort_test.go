package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	domain "alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/logging"

)

type toolFailureBurstExecutor struct {
	canceledOnce sync.Once
	canceledCh   chan struct{}
}

func (e *toolFailureBurstExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *toolFailureBurstExecutor) ExecuteTask(ctx context.Context, _ string, sessionID string, listener agentports.EventListener) (*agentports.TaskResult, error) {
	base := domain.NewBaseEvent(agentports.LevelCore, sessionID, "run-"+sessionID, "", time.Now())
	for i := 0; i < 16; i++ {
		if listener != nil {
			listener.OnEvent(domain.NewToolCompletedEvent(
				base,
				fmt.Sprintf("call-%d", i),
				"bash",
				"",
				fmt.Errorf("boom-%d", i),
				10*time.Millisecond,
				nil,
				nil,
			))
		}
		select {
		case <-ctx.Done():
			e.markCanceled()
			return nil, ctx.Err()
		default:
		}
	}
	<-ctx.Done()
	e.markCanceled()
	return nil, ctx.Err()
}

func (e *toolFailureBurstExecutor) markCanceled() {
	e.canceledOnce.Do(func() { close(e.canceledCh) })
}

func TestHandleMessageToolFailureThresholdSendsAbortNotice(t *testing.T) {
	executor := &toolFailureBurstExecutor{canceledCh: make(chan struct{})}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "lark",
				AllowDirect:   true,
				ReplyTimeout:  time.Minute,
			},
			AppID:                     "test",
			AppSecret:                 "secret",
			ToolFailureAbortThreshold: 3,
		},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: &strictContextMessenger{inner: recorder},
		now:       time.Now,
	}
		gw.dedup = newEventDedup(nil)

	if err := gw.InjectMessage(context.Background(), "oc_tool_fail", "p2p", "ou_sender", "om_tool_fail", "run task"); err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	select {
	case <-executor.canceledCh:
	case <-time.After(2 * time.Second):
		t.Fatal("expected task context cancellation after repeated tool failures")
	}
	gw.WaitForTasks()

	replies := recorder.CallsByMethod(MethodReplyMessage)
	if len(replies) == 0 {
		t.Fatal("expected visible abort reply")
	}
	foundAbort := false
	for _, reply := range replies {
		text := extractTextContent(reply.Content, nil)
		if strings.Contains(text, "连续失败 3 次") && strings.Contains(text, "已自动中止") {
			foundAbort = true
			break
		}
	}
	if !foundAbort {
		t.Fatalf("expected abort notice in replies, got %#v", replies)
	}
}
