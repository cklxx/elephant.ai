package notification

import (
	"context"
	"errors"
	"sync"
	"testing"

	"alex/internal/shared/notification"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockNotifier records Send calls.
type mockNotifier struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (m *mockNotifier) Send(_ context.Context, _ notification.Target, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	return m.err
}

// mockRecorder records alert outcomes.
type mockRecorder struct {
	mu       sync.Mutex
	outcomes []recordedOutcome
}

type recordedOutcome struct {
	Feature string
	Channel string
	Outcome notification.AlertOutcome
}

func (r *mockRecorder) RecordAlertOutcome(_ context.Context, feature, channel string, outcome notification.AlertOutcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outcomes = append(r.outcomes, recordedOutcome{Feature: feature, Channel: channel, Outcome: outcome})
}

func (r *mockRecorder) getOutcomes() []recordedOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedOutcome, len(r.outcomes))
	copy(out, r.outcomes)
	return out
}

// mockLatency records latency calls.
type mockLatency struct {
	mu       sync.Mutex
	calls    int
	lastMs   float64
	feature  string
	channel  string
}

func (l *mockLatency) RecordAlertSendLatency(_ context.Context, feature, channel string, latencyMs float64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls++
	l.lastMs = latencyMs
	l.feature = feature
	l.channel = channel
}

func TestInstrumentedNotifier_SuccessfulSend(t *testing.T) {
	inner := &mockNotifier{}
	recorder := &mockRecorder{}
	latency := &mockLatency{}
	n := NewInstrumentedNotifier(inner, recorder, latency, "blocker_radar")

	target := notification.Target{Channel: "lark", ChatID: "oc_test"}
	err := n.Send(context.Background(), target, "alert content")
	require.NoError(t, err)

	assert.Equal(t, 1, inner.calls)

	outcomes := recorder.getOutcomes()
	require.Len(t, outcomes, 2)
	assert.Equal(t, recordedOutcome{"blocker_radar", "lark", notification.OutcomeSent}, outcomes[0])
	assert.Equal(t, recordedOutcome{"blocker_radar", "lark", notification.OutcomeDelivered}, outcomes[1])

	assert.Equal(t, 1, latency.calls)
	assert.Equal(t, "blocker_radar", latency.feature)
	assert.Equal(t, "lark", latency.channel)
	assert.GreaterOrEqual(t, latency.lastMs, 0.0)
}

func TestInstrumentedNotifier_FailedSend(t *testing.T) {
	inner := &mockNotifier{err: errors.New("lark unavailable")}
	recorder := &mockRecorder{}
	n := NewInstrumentedNotifier(inner, recorder, nil, "weekly_pulse")

	target := notification.Target{Channel: "lark", ChatID: "oc_test"}
	err := n.Send(context.Background(), target, "digest")
	require.Error(t, err)

	outcomes := recorder.getOutcomes()
	require.Len(t, outcomes, 2)
	assert.Equal(t, notification.OutcomeSent, outcomes[0].Outcome)
	assert.Equal(t, notification.OutcomeFailed, outcomes[1].Outcome)
}

func TestInstrumentedNotifier_NilRecorder(t *testing.T) {
	inner := &mockNotifier{}
	n := NewInstrumentedNotifier(inner, nil, nil, "milestone_checkin")

	target := notification.Target{Channel: "lark", ChatID: "oc_test"}
	err := n.Send(context.Background(), target, "snapshot")
	require.NoError(t, err)
	assert.Equal(t, 1, inner.calls)
}

func TestInstrumentedNotifier_RecordOutcome(t *testing.T) {
	recorder := &mockRecorder{}
	n := NewInstrumentedNotifier(NopNotifier{}, recorder, nil, "blocker_radar")

	n.RecordOutcome(context.Background(), "lark", notification.OutcomeOpened)
	n.RecordOutcome(context.Background(), "lark", notification.OutcomeDismissed)
	n.RecordOutcome(context.Background(), "lark", notification.OutcomeActedOn)

	outcomes := recorder.getOutcomes()
	require.Len(t, outcomes, 3)
	assert.Equal(t, notification.OutcomeOpened, outcomes[0].Outcome)
	assert.Equal(t, notification.OutcomeDismissed, outcomes[1].Outcome)
	assert.Equal(t, notification.OutcomeActedOn, outcomes[2].Outcome)
}

func TestInstrumentedNotifier_RecordOutcome_NilRecorder(t *testing.T) {
	n := NewInstrumentedNotifier(NopNotifier{}, nil, nil, "prep_brief")
	// Should not panic.
	n.RecordOutcome(context.Background(), "lark", notification.OutcomeOpened)
}

func TestInstrumentedNotifier_FeatureLabel(t *testing.T) {
	features := []string{"blocker_radar", "weekly_pulse", "milestone_checkin", "prep_brief"}
	for _, feature := range features {
		recorder := &mockRecorder{}
		n := NewInstrumentedNotifier(NopNotifier{}, recorder, nil, feature)

		target := notification.Target{Channel: "lark", ChatID: "oc_test"}
		_ = n.Send(context.Background(), target, "test")

		outcomes := recorder.getOutcomes()
		for _, o := range outcomes {
			assert.Equal(t, feature, o.Feature, "feature label mismatch for %s", feature)
		}
	}
}
