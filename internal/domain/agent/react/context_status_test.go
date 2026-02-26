package react

import (
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
	tokenutil "alex/internal/shared/token"
)

func TestDeriveContextPhase(t *testing.T) {
	cases := []struct {
		name                string
		ratio               float64
		compressionOccurred bool
		compactionSeq       int
		want                string
	}{
		{"low usage", 0.30, false, 0, contextPhaseOK},
		{"just below warning", 0.69, false, 0, contextPhaseOK},
		{"at warning threshold", 0.70, false, 0, contextPhaseWarning},
		{"between warning and compressed", 0.80, false, 0, contextPhaseWarning},
		{"at compressed threshold", 0.85, false, 0, contextPhaseCompressed},
		{"above compressed threshold", 0.95, false, 0, contextPhaseCompressed},
		{"compression occurred overrides ratio", 0.50, true, 0, contextPhaseCompressed},
		{"compaction seq > 1 forces trimmed", 0.50, false, 2, contextPhaseTrimmed},
		{"compaction seq > 1 overrides high ratio", 0.95, true, 3, contextPhaseTrimmed},
		{"compaction seq == 1 does not force trimmed", 0.50, false, 1, contextPhaseOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := deriveContextPhase(tc.ratio, tc.compressionOccurred, tc.compactionSeq)
			if got != tc.want {
				t.Fatalf("deriveContextPhase(%.2f, %v, %d) = %q, want %q",
					tc.ratio, tc.compressionOccurred, tc.compactionSeq, got, tc.want)
			}
		})
	}
}

func TestShouldInjectContextStatus(t *testing.T) {
	cases := []struct {
		phase string
		want  bool
	}{
		{contextPhaseOK, false},
		{contextPhaseWarning, true},
		{contextPhaseCompressed, true},
		{contextPhaseTrimmed, true},
	}
	for _, tc := range cases {
		got := shouldInjectContextStatus(ContextBudgetStatus{Phase: tc.phase})
		if got != tc.want {
			t.Fatalf("shouldInjectContextStatus(phase=%q) = %v, want %v", tc.phase, got, tc.want)
		}
	}
}

func TestBuildContextStatusMessage_Warning(t *testing.T) {
	status := ContextBudgetStatus{Phase: contextPhaseWarning, UsagePercent: 75}
	msg := buildContextStatusMessage(status)

	if !strings.Contains(msg.Content, `phase="warning"`) {
		t.Fatalf("expected phase=warning, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, `usage="75%"`) {
		t.Fatalf("expected usage=75%%, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, warningDirective) {
		t.Fatalf("expected warning directive, got: %s", msg.Content)
	}
}

func TestBuildContextStatusMessage_Compressed(t *testing.T) {
	status := ContextBudgetStatus{Phase: contextPhaseCompressed, UsagePercent: 88}
	msg := buildContextStatusMessage(status)

	if !strings.Contains(msg.Content, `phase="compressed"`) {
		t.Fatalf("expected phase=compressed, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, compressedDirective) {
		t.Fatalf("expected compressed directive, got: %s", msg.Content)
	}
}

func TestBuildContextStatusMessage_Trimmed(t *testing.T) {
	status := ContextBudgetStatus{Phase: contextPhaseTrimmed, UsagePercent: 94}
	msg := buildContextStatusMessage(status)

	if !strings.Contains(msg.Content, `phase="trimmed"`) {
		t.Fatalf("expected phase=trimmed, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, trimmedDirective) {
		t.Fatalf("expected trimmed directive, got: %s", msg.Content)
	}
}

func TestContextStatusMessageTokenCost(t *testing.T) {
	// Worst case: trimmed phase with directive.
	status := ContextBudgetStatus{UsagePercent: 94, Phase: contextPhaseTrimmed}
	msg := buildContextStatusMessage(status)
	tokens := tokenutil.CountTokens(msg.Content)
	t.Logf("worst-case: %d tokens, content: %s", tokens, msg.Content)

	if tokens > 35 {
		t.Fatalf("context status costs %d tokens (>35), content: %s", tokens, msg.Content)
	}
}

func TestBuildContextBudgetStatus_NilState(t *testing.T) {
	status := buildContextBudgetStatus(50000, 125000, nil, false)

	if status.Phase != contextPhaseOK {
		t.Fatalf("expected phase=ok for 40%% usage, got %q", status.Phase)
	}
}

func TestBuildContextBudgetStatus_CompactionSeqTrimmed(t *testing.T) {
	state := &agent.TaskState{ContextCompactionSeq: 2}
	status := buildContextBudgetStatus(100000, 125000, state, false)

	if status.Phase != contextPhaseTrimmed {
		t.Fatalf("expected phase=trimmed for compactionSeq=2, got %q", status.Phase)
	}
}
