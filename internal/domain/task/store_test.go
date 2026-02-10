package task

import (
	"testing"
)

func TestStatusIsTerminal(t *testing.T) {
	tests := []struct {
		status   Status
		terminal bool
	}{
		{StatusPending, false},
		{StatusRunning, false},
		{StatusWaitingInput, false},
		{StatusCompleted, true},
		{StatusFailed, true},
		{StatusCancelled, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsTerminal(); got != tt.terminal {
				t.Errorf("Status(%q).IsTerminal() = %v, want %v", tt.status, got, tt.terminal)
			}
		})
	}
}

func TestApplyTransitionOptions(t *testing.T) {
	preview := "test preview"
	errText := "test error"
	tokens := 1000

	opts := []TransitionOption{
		WithTransitionReason("testing"),
		WithTransitionAnswerPreview(preview),
		WithTransitionError(errText),
		WithTransitionTokens(tokens),
		WithTransitionMeta(map[string]any{"key": "value"}),
	}

	p := ApplyTransitionOptions(opts)

	if p.Reason != "testing" {
		t.Errorf("Reason = %q, want %q", p.Reason, "testing")
	}
	if p.AnswerPreview == nil || *p.AnswerPreview != preview {
		t.Errorf("AnswerPreview = %v, want %q", p.AnswerPreview, preview)
	}
	if p.ErrorText == nil || *p.ErrorText != errText {
		t.Errorf("ErrorText = %v, want %q", p.ErrorText, errText)
	}
	if p.TokensUsed == nil || *p.TokensUsed != tokens {
		t.Errorf("TokensUsed = %v, want %d", p.TokensUsed, tokens)
	}
	if p.Metadata["key"] != "value" {
		t.Errorf("Metadata = %v, want key=value", p.Metadata)
	}
}

func TestApplyTransitionOptions_Empty(t *testing.T) {
	p := ApplyTransitionOptions(nil)

	if p.Reason != "" {
		t.Errorf("Reason = %q, want empty", p.Reason)
	}
	if p.AnswerPreview != nil {
		t.Errorf("AnswerPreview = %v, want nil", p.AnswerPreview)
	}
	if p.ErrorText != nil {
		t.Errorf("ErrorText = %v, want nil", p.ErrorText)
	}
	if p.TokensUsed != nil {
		t.Errorf("TokensUsed = %v, want nil", p.TokensUsed)
	}
}
