package digest

import (
	"context"
	"errors"
	"testing"
	"time"

	"alex/internal/shared/notification"
)

type mockSpec struct {
	name      string
	content   *Content
	genErr    error
	formatted string
}

func (m *mockSpec) Name() string                           { return m.name }
func (m *mockSpec) Generate(_ context.Context) (*Content, error) { return m.content, m.genErr }
func (m *mockSpec) Format(_ *Content) string               { return m.formatted }

type mockNotifier struct {
	sent    []string
	sendErr error
}

func (m *mockNotifier) Send(_ context.Context, _ notification.Target, content string) error {
	m.sent = append(m.sent, content)
	return m.sendErr
}

type mockRecorder struct {
	outcomes []notification.AlertOutcome
}

func (m *mockRecorder) RecordAlertOutcome(_ context.Context, _, _ string, outcome notification.AlertOutcome) {
	m.outcomes = append(m.outcomes, outcome)
}

func TestServiceRun(t *testing.T) {
	tests := []struct {
		name        string
		spec        *mockSpec
		notifier    *mockNotifier
		recorder    *mockRecorder
		wantErr     bool
		wantSent    []string
		wantOutcome notification.AlertOutcome
	}{
		{
			name: "happy path",
			spec: &mockSpec{
				name:      "test-digest",
				content:   &Content{Title: "Test"},
				formatted: "# Test",
			},
			notifier:    &mockNotifier{},
			recorder:    &mockRecorder{},
			wantSent:    []string{"# Test"},
			wantOutcome: notification.OutcomeSent,
		},
		{
			name: "generate error",
			spec: &mockSpec{
				name:   "fail-gen",
				genErr: errors.New("db down"),
			},
			notifier:    &mockNotifier{},
			recorder:    &mockRecorder{},
			wantErr:     true,
			wantOutcome: notification.OutcomeFailed,
		},
		{
			name: "send error",
			spec: &mockSpec{
				name:      "fail-send",
				content:   &Content{Title: "Test"},
				formatted: "# Test",
			},
			notifier:    &mockNotifier{sendErr: errors.New("network")},
			recorder:    &mockRecorder{},
			wantErr:     true,
			wantOutcome: notification.OutcomeFailed,
		},
		{
			name: "nil recorder",
			spec: &mockSpec{
				name:      "no-recorder",
				content:   &Content{Title: "Test"},
				formatted: "# Test",
			},
			notifier: &mockNotifier{},
			recorder: nil,
			wantSent: []string{"# Test"},
		},
		{
			name: "empty content",
			spec: &mockSpec{
				name:      "empty",
				content:   &Content{},
				formatted: "",
			},
			notifier:    &mockNotifier{},
			recorder:    &mockRecorder{},
			wantSent:    []string{""},
			wantOutcome: notification.OutcomeSent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rec notification.OutcomeRecorder
			if tt.recorder != nil {
				rec = tt.recorder
			}
			svc := NewService(tt.notifier, notification.Target{Channel: "test"}, rec, time.Now)
			err := svc.Run(context.Background(), tt.spec)

			if (err != nil) != tt.wantErr {
				t.Fatalf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantSent != nil {
				if len(tt.notifier.sent) != len(tt.wantSent) {
					t.Fatalf("sent count = %d, want %d", len(tt.notifier.sent), len(tt.wantSent))
				}
				for i, want := range tt.wantSent {
					if tt.notifier.sent[i] != want {
						t.Errorf("sent[%d] = %q, want %q", i, tt.notifier.sent[i], want)
					}
				}
			}
			if tt.recorder != nil && tt.wantOutcome != "" {
				if len(tt.recorder.outcomes) == 0 {
					t.Fatal("no outcomes recorded")
				}
				got := tt.recorder.outcomes[len(tt.recorder.outcomes)-1]
				if got != tt.wantOutcome {
					t.Errorf("outcome = %q, want %q", got, tt.wantOutcome)
				}
			}
		})
	}
}
