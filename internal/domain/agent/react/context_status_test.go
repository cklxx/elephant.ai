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

func TestBuildContextStatusMessage_OK(t *testing.T) {
	status := ContextBudgetStatus{
		TokensUsed:    45000,
		TokenLimit:    125000,
		UsagePercent:  36,
		Phase:         contextPhaseOK,
		Iteration:     3,
		MaxIterations: 25,
		MessageCount:  12,
	}

	msg := buildContextStatusMessage(status)

	if msg.Role != "system" {
		t.Fatalf("expected role=system, got %q", msg.Role)
	}
	if !strings.Contains(msg.Content, `p="ok"`) {
		t.Fatalf("expected p=ok in content, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, `t="3/25"`) {
		t.Fatalf("expected t=3/25 in content, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, `tk="45000/125000"`) {
		t.Fatalf("expected tk in content, got: %s", msg.Content)
	}
	// No directive for "ok" phase.
	if strings.Contains(msg.Content, warningDirective) ||
		strings.Contains(msg.Content, compressedDirective) ||
		strings.Contains(msg.Content, trimmedDirective) {
		t.Fatalf("expected no directive for ok phase, got: %s", msg.Content)
	}
}

func TestBuildContextStatusMessage_Warning(t *testing.T) {
	status := ContextBudgetStatus{
		Phase: contextPhaseWarning,
	}

	msg := buildContextStatusMessage(status)

	if !strings.Contains(msg.Content, `p="warning"`) {
		t.Fatalf("expected p=warning, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, warningDirective) {
		t.Fatalf("expected warning directive, got: %s", msg.Content)
	}
}

func TestBuildContextStatusMessage_Compressed(t *testing.T) {
	status := ContextBudgetStatus{
		Phase:               contextPhaseCompressed,
		CompressionOccurred: true,
	}

	msg := buildContextStatusMessage(status)

	if !strings.Contains(msg.Content, `p="compressed"`) {
		t.Fatalf("expected p=compressed, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, compressedDirective) {
		t.Fatalf("expected compressed directive, got: %s", msg.Content)
	}
}

func TestBuildContextStatusMessage_Trimmed(t *testing.T) {
	status := ContextBudgetStatus{
		Phase: contextPhaseTrimmed,
	}

	msg := buildContextStatusMessage(status)

	if !strings.Contains(msg.Content, `p="trimmed"`) {
		t.Fatalf("expected p=trimmed, got: %s", msg.Content)
	}
	if !strings.Contains(msg.Content, trimmedDirective) {
		t.Fatalf("expected trimmed directive, got: %s", msg.Content)
	}
}

func TestContextStatusMessageTokenCost(t *testing.T) {
	// Worst case: trimmed phase with directive (longest content).
	status := ContextBudgetStatus{
		TokensUsed:          120000,
		TokenLimit:          128000,
		UsagePercent:        93.75,
		Phase:               contextPhaseTrimmed,
		Iteration:           25,
		MaxIterations:       25,
		CompressionOccurred: true,
		PendingSummary:      true,
		MessageCount:        48,
	}

	msg := buildContextStatusMessage(status)
	tokens := tokenutil.CountTokens(msg.Content)
	t.Logf("worst-case token cost: %d, content: %s", tokens, msg.Content)

	if tokens > 60 {
		t.Fatalf("context status message costs %d tokens (>60), content: %s", tokens, msg.Content)
	}
}

func TestContextStatusMessageTokenCost_OK(t *testing.T) {
	// Best case: ok phase, no directive.
	status := ContextBudgetStatus{
		TokensUsed:    45000,
		TokenLimit:    125000,
		UsagePercent:  36,
		Phase:         contextPhaseOK,
		Iteration:     3,
		MaxIterations: 25,
		MessageCount:  12,
	}

	msg := buildContextStatusMessage(status)
	tokens := tokenutil.CountTokens(msg.Content)
	t.Logf("ok-phase token cost: %d, content: %s", tokens, msg.Content)

	if tokens > 30 {
		t.Fatalf("ok-phase context status costs %d tokens (>30), content: %s", tokens, msg.Content)
	}
}

func TestBuildContextBudgetStatus_NilState(t *testing.T) {
	status := buildContextBudgetStatus(50000, 125000, nil, 25, false, 10)

	if status.Iteration != 0 {
		t.Fatalf("expected iteration=0 for nil state, got %d", status.Iteration)
	}
	if status.Phase != contextPhaseOK {
		t.Fatalf("expected phase=ok for 40%% usage, got %q", status.Phase)
	}
}

func TestBuildContextBudgetStatus_WithState(t *testing.T) {
	state := &agent.TaskState{
		Iterations:           5,
		ContextCompactionSeq: 2,
		PendingSummary:       "some summary",
	}

	status := buildContextBudgetStatus(100000, 125000, state, 25, false, 20)

	if status.Iteration != 5 {
		t.Fatalf("expected iteration=5, got %d", status.Iteration)
	}
	if !status.PendingSummary {
		t.Fatal("expected PendingSummary=true")
	}
	// CompactionSeq > 1 → trimmed, regardless of ratio.
	if status.Phase != contextPhaseTrimmed {
		t.Fatalf("expected phase=trimmed for compactionSeq=2, got %q", status.Phase)
	}
}
