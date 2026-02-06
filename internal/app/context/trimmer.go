package context

import (
	"alex/internal/agent/ports"
	"alex/internal/tokenutil"
)

// ModelCostProfile captures per-model pricing and context-window limits used for
// cost-aware trimming decisions.
type ModelCostProfile struct {
	Name            string
	InputCostPer1K  float64
	OutputCostPer1K float64
	ContextWindow   int
}

// DefaultModelProfiles provides cost profiles for commonly used models. Prices
// are expressed as USD per 1 000 input/output tokens.
var DefaultModelProfiles = map[string]ModelCostProfile{
	"gpt-4":          {Name: "gpt-4", InputCostPer1K: 0.03, OutputCostPer1K: 0.06, ContextWindow: 8192},
	"gpt-3.5-turbo":  {Name: "gpt-3.5-turbo", InputCostPer1K: 0.0005, OutputCostPer1K: 0.0015, ContextWindow: 16384},
	"claude-3-opus":   {Name: "claude-3-opus", InputCostPer1K: 0.015, OutputCostPer1K: 0.075, ContextWindow: 200000},
	"claude-3-sonnet": {Name: "claude-3-sonnet", InputCostPer1K: 0.003, OutputCostPer1K: 0.015, ContextWindow: 200000},
	"deepseek-chat":   {Name: "deepseek-chat", InputCostPer1K: 0.0014, OutputCostPer1K: 0.0028, ContextWindow: 128000},
}

// TrimConfig controls how TrimMessages selects which messages to keep.
type TrimConfig struct {
	// MaxTokens is the hard token budget for kept messages. Required (> 0).
	MaxTokens int
	// MaxCostUSD is an optional cost ceiling in USD. When > 0 and Model is set,
	// messages are also trimmed to stay within this estimated input cost.
	MaxCostUSD float64
	// PreservedSources lists message sources that must never be trimmed
	// (e.g. SystemPrompt, Important).
	PreservedSources []ports.MessageSource
	// Model provides optional cost information for cost-aware trimming.
	Model *ModelCostProfile
}

// TrimResult captures the outcome of a TrimMessages call.
type TrimResult struct {
	Kept             []ports.Message
	Trimmed          []ports.Message
	TotalTokens      int
	EstimatedCostUSD float64
}

// sourcePriority returns a numeric priority for each MessageSource. Higher
// values indicate higher priority (trimmed last). The ordering mirrors C32:
//
//	SystemPrompt > Important > UserInput > Proactive > AssistantReply > ToolResult > UserHistory > Debug/Evaluation
func sourcePriority(src ports.MessageSource) int {
	switch src {
	case ports.MessageSourceSystemPrompt:
		return 8
	case ports.MessageSourceImportant:
		return 7
	case ports.MessageSourceUserInput:
		return 6
	case ports.MessageSourceProactive:
		return 5
	case ports.MessageSourceAssistantReply:
		return 4
	case ports.MessageSourceToolResult:
		return 3
	case ports.MessageSourceUserHistory:
		return 2
	case ports.MessageSourceDebug, ports.MessageSourceEvaluation:
		return 1
	default:
		return 0
	}
}

// EstimateInputCost computes the estimated input cost in USD for the given
// token count using the supplied model profile. Returns 0 if model is nil.
func EstimateInputCost(tokens int, model *ModelCostProfile) float64 {
	if model == nil {
		return 0
	}
	return float64(tokens) / 1000.0 * model.InputCostPer1K
}

// trimEntry holds pre-computed metadata for a single message during trimming.
type trimEntry struct {
	index     int
	msg       ports.Message
	tokens    int
	preserved bool
	priority  int
}

// trimCandidate identifies a trimmable message by its index and priority.
type trimCandidate struct {
	entryIdx int
	priority int
}

// TrimMessages selectively removes lower-priority messages so the total token
// count stays within config.MaxTokens (and optionally within config.MaxCostUSD).
//
// Messages whose Source appears in config.PreservedSources are never removed,
// even if they alone exceed the budget. The returned Kept slice maintains the
// original message order.
func TrimMessages(messages []ports.Message, config TrimConfig) TrimResult {
	if len(messages) == 0 {
		return TrimResult{}
	}

	preservedSet := make(map[ports.MessageSource]struct{}, len(config.PreservedSources))
	for _, src := range config.PreservedSources {
		preservedSet[src] = struct{}{}
	}

	// Pre-compute token counts and classify each message.
	entries := make([]trimEntry, len(messages))
	for i, msg := range messages {
		_, preserved := preservedSet[msg.Source]
		entries[i] = trimEntry{
			index:     i,
			msg:       msg,
			tokens:    tokenutil.CountTokens(msg.Content),
			preserved: preserved,
			priority:  sourcePriority(msg.Source),
		}
	}

	// Start with all messages kept.
	kept := make([]bool, len(entries))
	totalTokens := 0
	for i := range entries {
		kept[i] = true
		totalTokens += entries[i].tokens
	}

	budget := config.MaxTokens
	if budget <= 0 {
		// No budget constraint — keep everything.
		return buildTrimResult(entries, kept, totalTokens, config.Model)
	}

	// Check if we need to trim at all.
	if withinBudget(totalTokens, budget, config.MaxCostUSD, config.Model) {
		return buildTrimResult(entries, kept, totalTokens, config.Model)
	}

	// Build a list of trimmable indices sorted by priority ASC (lowest first).
	// Within the same priority, later messages are trimmed first (reverse order)
	// to keep earlier conversational context when possible.
	var candidates []trimCandidate
	for i, e := range entries {
		if !e.preserved {
			candidates = append(candidates, trimCandidate{entryIdx: i, priority: e.priority})
		}
	}
	sortTrimCandidates(candidates)

	// Remove candidates one by one until within budget.
	for _, c := range candidates {
		if withinBudget(totalTokens, budget, config.MaxCostUSD, config.Model) {
			break
		}
		kept[c.entryIdx] = false
		totalTokens -= entries[c.entryIdx].tokens
	}

	return buildTrimResult(entries, kept, totalTokens, config.Model)
}

// withinBudget checks both token and optional cost constraints.
func withinBudget(tokens, maxTokens int, maxCostUSD float64, model *ModelCostProfile) bool {
	if tokens > maxTokens {
		return false
	}
	if maxCostUSD > 0 && model != nil {
		if EstimateInputCost(tokens, model) > maxCostUSD {
			return false
		}
	}
	return true
}

// sortTrimCandidates sorts by priority ASC, then by entryIdx DESC (later
// messages trimmed first within the same priority tier). Uses insertion sort
// for simplicity — the number of trimmable messages is typically small.
func sortTrimCandidates(cs []trimCandidate) {
	for i := 1; i < len(cs); i++ {
		key := cs[i]
		j := i - 1
		for j >= 0 && trimCandidateLess(key, cs[j]) {
			cs[j+1] = cs[j]
			j--
		}
		cs[j+1] = key
	}
}

func trimCandidateLess(a, b trimCandidate) bool {
	if a.priority != b.priority {
		return a.priority < b.priority
	}
	return a.entryIdx > b.entryIdx // later messages first within same priority
}

// buildTrimResult assembles the final TrimResult preserving original message order.
func buildTrimResult(entries []trimEntry, kept []bool, totalTokens int, model *ModelCostProfile) TrimResult {
	var keptMsgs, trimmedMsgs []ports.Message
	for i, e := range entries {
		if kept[i] {
			keptMsgs = append(keptMsgs, e.msg)
		} else {
			trimmedMsgs = append(trimmedMsgs, e.msg)
		}
	}
	return TrimResult{
		Kept:             keptMsgs,
		Trimmed:          trimmedMsgs,
		TotalTokens:      totalTokens,
		EstimatedCostUSD: EstimateInputCost(totalTokens, model),
	}
}
