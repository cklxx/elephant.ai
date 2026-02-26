package react

import (
	"fmt"

	"alex/internal/domain/agent/ports"
)

// ContextBudgetStatus captures the current token budget state for LLM self-awareness.
type ContextBudgetStatus struct {
	TokensUsed          int
	TokenLimit          int
	UsagePercent        float64
	Phase               string // "ok", "warning", "compressed", "trimmed"
	Iteration           int
	MaxIterations       int
	CompressionOccurred bool
	PendingSummary      bool
	MessageCount        int
}

// Context budget phase thresholds.
const (
	contextPhaseOK         = "ok"
	contextPhaseWarning    = "warning"
	contextPhaseCompressed = "compressed"
	contextPhaseTrimmed    = "trimmed"

	warningThreshold    = 0.70
	compressedThreshold = 0.85
)

// Phase-graduated behavioral directives.
const (
	warningDirective    = "Context approaching capacity. Prefer concise responses; summarize rather than repeat known context."
	compressedDirective = "Earlier conversation was compressed. Working context may be incomplete. Verify assumptions before acting on historical details."
	trimmedDirective    = "Context was aggressively trimmed. Significant history lost. State critical context explicitly; avoid referencing earlier conversation without verification."
)

func deriveContextPhase(ratio float64, compressionOccurred bool, compactionSeq int) string {
	if compactionSeq > 1 {
		return contextPhaseTrimmed
	}
	if ratio >= compressedThreshold || compressionOccurred {
		return contextPhaseCompressed
	}
	if ratio >= warningThreshold {
		return contextPhaseWarning
	}
	return contextPhaseOK
}

func buildContextBudgetStatus(
	tokensUsed, tokenLimit int,
	state *TaskState,
	maxIterations int,
	compressionOccurred bool,
	messageCount int,
) ContextBudgetStatus {
	ratio := 0.0
	if tokenLimit > 0 {
		ratio = float64(tokensUsed) / float64(tokenLimit)
	}

	iteration := 0
	compactionSeq := 0
	pendingSummary := false
	if state != nil {
		iteration = state.Iterations
		compactionSeq = state.ContextCompactionSeq
		pendingSummary = state.PendingSummary != ""
	}

	return ContextBudgetStatus{
		TokensUsed:          tokensUsed,
		TokenLimit:          tokenLimit,
		UsagePercent:        ratio * 100,
		Phase:               deriveContextPhase(ratio, compressionOccurred, compactionSeq),
		Iteration:           iteration,
		MaxIterations:       maxIterations,
		CompressionOccurred: compressionOccurred,
		PendingSummary:      pendingSummary,
		MessageCount:        messageCount,
	}
}

func buildContextStatusMessage(status ContextBudgetStatus) ports.Message {
	tag := fmt.Sprintf(
		`<context_status turn="%d/%d" tokens="%d/%d" usage="%.0f%%" phase="%s" compressed="%t" pending_summary="%t" messages="%d"/>`,
		status.Iteration,
		status.MaxIterations,
		status.TokensUsed,
		status.TokenLimit,
		status.UsagePercent,
		status.Phase,
		status.CompressionOccurred,
		status.PendingSummary,
		status.MessageCount,
	)

	directive := phaseDirective(status.Phase)
	content := tag
	if directive != "" {
		content = tag + "\n" + directive
	}

	return ports.Message{
		Role:    "system",
		Content: content,
		Source:  ports.MessageSourceProactive,
	}
}

func phaseDirective(phase string) string {
	switch phase {
	case contextPhaseWarning:
		return warningDirective
	case contextPhaseCompressed:
		return compressedDirective
	case contextPhaseTrimmed:
		return trimmedDirective
	default:
		return ""
	}
}
