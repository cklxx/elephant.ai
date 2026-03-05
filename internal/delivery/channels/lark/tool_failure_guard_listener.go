package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

type toolFailureGuardListener struct {
	inner     agent.EventListener
	cancel    context.CancelFunc
	threshold int
	state     *toolFailureGuardState
}

type toolFailureGuardState struct {
	threshold int

	mu              sync.Mutex
	consecutiveFail int
	tripped         bool
}

func newToolFailureGuardListener(
	inner agent.EventListener,
	threshold int,
	cancel context.CancelFunc,
) (*toolFailureGuardListener, *toolFailureGuardState) {
	state := &toolFailureGuardState{threshold: threshold}
	return &toolFailureGuardListener{
		inner:     inner,
		cancel:    cancel,
		threshold: threshold,
		state:     state,
	}, state
}

func (l *toolFailureGuardListener) OnEvent(event agent.AgentEvent) {
	if l.inner != nil {
		l.inner.OnEvent(event)
	}

	failed, ok := isToolCompletedFailureEvent(event)
	if !ok {
		return
	}
	if !l.state.record(failed) {
		return
	}
	if l.cancel != nil {
		l.cancel()
	}
}

func isToolCompletedFailureEvent(event agent.AgentEvent) (failed, ok bool) {
	switch e := event.(type) {
	case *domain.Event:
		if e == nil || e.Kind != types.EventToolCompleted {
			return false, false
		}
		return e.Data.Error != nil || strings.TrimSpace(e.Data.ErrorStr) != "", true
	case *domain.WorkflowEventEnvelope:
		if e == nil || strings.TrimSpace(e.Event) != types.EventToolCompleted {
			return false, false
		}
		return envelopeHasError(e), true
	default:
		return false, false
	}
}

func (s *toolFailureGuardState) record(failed bool) bool {
	if s == nil || s.threshold <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tripped {
		return false
	}
	if failed {
		s.consecutiveFail++
		if s.consecutiveFail >= s.threshold {
			s.tripped = true
			return true
		}
		return false
	}
	s.consecutiveFail = 0
	return false
}

func (s *toolFailureGuardState) Tripped() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tripped
}

func (s *toolFailureGuardState) UserNotice() string {
	threshold := 0
	if s != nil {
		threshold = s.threshold
	}
	if threshold <= 0 {
		threshold = 1
	}
	return fmt.Sprintf("检测到工具已连续失败 %d 次，已自动中止本次执行，避免无效空转。请稍后重试，或回复“诊断”让我输出定位信息。", threshold)
}
