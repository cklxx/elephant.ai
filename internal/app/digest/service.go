package digest

import (
	"context"
	"fmt"
	"time"

	"alex/internal/shared/notification"
)

// Service orchestrates digest generation and delivery.
type Service struct {
	notifier notification.Notifier
	target   notification.Target
	recorder notification.OutcomeRecorder
	nowFn    func() time.Time
}

// NewService creates a Service with the given dependencies.
func NewService(
	notifier notification.Notifier,
	target notification.Target,
	recorder notification.OutcomeRecorder,
	nowFn func() time.Time,
) *Service {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Service{
		notifier: notifier,
		target:   target,
		recorder: recorder,
		nowFn:    nowFn,
	}
}

// Run executes a DigestSpec: generate, format, deliver, record.
func (s *Service) Run(ctx context.Context, spec DigestSpec) error {
	content, err := spec.Generate(ctx)
	if err != nil {
		s.recordOutcome(ctx, spec.Name(), notification.OutcomeFailed)
		return fmt.Errorf("digest %s generate: %w", spec.Name(), err)
	}
	formatted := spec.Format(content)
	if err := s.notifier.Send(ctx, s.target, formatted); err != nil {
		s.recordOutcome(ctx, spec.Name(), notification.OutcomeFailed)
		return fmt.Errorf("digest %s send: %w", spec.Name(), err)
	}
	s.recordOutcome(ctx, spec.Name(), notification.OutcomeSent)
	return nil
}

func (s *Service) recordOutcome(ctx context.Context, name string, outcome notification.AlertOutcome) {
	if s.recorder != nil {
		s.recorder.RecordAlertOutcome(ctx, name, s.target.Channel, outcome)
	}
}
