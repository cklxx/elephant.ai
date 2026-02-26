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
	CompressionOccurred bool
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

// Phase-graduated behavioral directives — kept short to minimize token cost.
const (
	warningDirective    = "Prefer concise responses; summarize rather than repeat known context."
	compressedDirective = "Context was compressed; verify assumptions before acting on historical details."
	trimmedDirective    = "Significant history lost; state critical context explicitly, do not reference earlier conversation."
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
	compressionOccurred bool,
) ContextBudgetStatus {
	ratio := 0.0
	if tokenLimit > 0 {
		ratio = float64(tokensUsed) / float64(tokenLimit)
	}

	compactionSeq := 0
	if state != nil {
		compactionSeq = state.ContextCompactionSeq
	}

	return ContextBudgetStatus{
		TokensUsed:          tokensUsed,
		TokenLimit:          tokenLimit,
		UsagePercent:        ratio * 100,
		Phase:               deriveContextPhase(ratio, compressionOccurred, compactionSeq),
		CompressionOccurred: compressionOccurred,
	}
}

// shouldInjectContextStatus returns true when the phase carries actionable
// information for the model. Phase "ok" is the default — injecting it every
// turn wastes tokens for zero behavioral change.
func shouldInjectContextStatus(status ContextBudgetStatus) bool {
	return status.Phase != contextPhaseOK
}

// buildContextStatusMessage produces a minimal system message only for non-ok
// phases. Format: `<ctx usage="75%" phase="warning"/>` + directive.
// Token cost: ~20–30 tokens (only when injected).
func buildContextStatusMessage(status ContextBudgetStatus) ports.Message {
	tag := fmt.Sprintf(`<ctx usage="%.0f%%" phase="%s"/>`, status.UsagePercent, status.Phase)

	content := tag + "\n" + phaseDirective(status.Phase)

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
