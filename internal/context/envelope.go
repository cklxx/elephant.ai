package context

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"alex/internal/agent/ports"
)

// SectionName enumerates logical context slices for budgeting and hashing.
type SectionName string

const (
	SectionSystem  SectionName = "system"
	SectionStatic  SectionName = "static"
	SectionDynamic SectionName = "dynamic"
	SectionMeta    SectionName = "meta"
)

// SectionBudget declares a max token budget for a section.
type SectionBudget struct {
	Limit int
}

// SectionBudgets wires budgets and a compression threshold.
// If Threshold is zero, the default 0.8 value is applied.
type SectionBudgets struct {
	Threshold float64
	System    SectionBudget
	Static    SectionBudget
	Dynamic   SectionBudget
	Meta      SectionBudget
}

// SectionBudgetStatus captures current usage and whether compression is advised.
type SectionBudgetStatus struct {
	Limit          int
	Threshold      float64
	Tokens         int
	ShouldCompress bool
}

// Envelope contains the layered context alongside budgeting metadata.
// It is purposefully hashable so caches can bind to the exact user + persona mix.
type Envelope struct {
	SessionID string
	UserRef   string

	Window ports.ContextWindow

	TokensBySection map[SectionName]int
	Budgets         map[SectionName]SectionBudgetStatus

	Hash string
}

// BudgetSummaries flattens budget status into the portable format exposed on
// ContextWindow for caches and UI consumption.
func (e Envelope) BudgetSummaries() []ports.SectionBudgetStatus {
	if len(e.Budgets) == 0 {
		return nil
	}
	summaries := make([]ports.SectionBudgetStatus, 0, len(e.Budgets))
	for name, status := range e.Budgets {
		summaries = append(summaries, ports.SectionBudgetStatus{
			Section:        string(name),
			Limit:          status.Limit,
			Threshold:      status.Threshold,
			Tokens:         status.Tokens,
			ShouldCompress: status.ShouldCompress,
		})
	}
	return summaries
}

// TokenSummaries normalizes section token counts for downstream consumers.
func (e Envelope) TokenSummaries() map[string]int {
	if len(e.TokensBySection) == 0 {
		return nil
	}
	normalized := make(map[string]int, len(e.TokensBySection))
	for name, tokens := range e.TokensBySection {
		normalized[string(name)] = tokens
	}
	return normalized
}

// BuildEnvelope constructs a hashable, budget-aware view of the context window.
func BuildEnvelope(window ports.ContextWindow, session *ports.Session, budgets SectionBudgets) Envelope {
	threshold := budgets.Threshold
	if threshold == 0 {
		threshold = defaultThreshold
	}

	tokenized := map[SectionName]int{
		SectionSystem:  estimateTokensForString(window.SystemPrompt),
		SectionStatic:  estimateTokensForStruct(window.Static),
		SectionDynamic: estimateTokensForStruct(window.Dynamic),
		SectionMeta:    estimateTokensForStruct(window.Meta),
	}

	budgetStatus := map[SectionName]SectionBudgetStatus{
		SectionSystem:  buildBudgetStatus(budgets.System, tokenized[SectionSystem], threshold),
		SectionStatic:  buildBudgetStatus(budgets.Static, tokenized[SectionStatic], threshold),
		SectionDynamic: buildBudgetStatus(budgets.Dynamic, tokenized[SectionDynamic], threshold),
		SectionMeta:    buildBudgetStatus(budgets.Meta, tokenized[SectionMeta], threshold),
	}

	userRef := bindUserRef(session)

	return Envelope{
		SessionID:       window.SessionID,
		UserRef:         userRef,
		Window:          window,
		TokensBySection: tokenized,
		Budgets:         budgetStatus,
		Hash:            computeEnvelopeHash(window, userRef),
	}
}

func bindUserRef(session *ports.Session) string {
	if session == nil {
		return ""
	}
	// Prefer explicit user identifiers when present.
	if session.Metadata != nil {
		if userID := strings.TrimSpace(session.Metadata["user_id"]); userID != "" {
			return userID
		}
		if userID := strings.TrimSpace(session.Metadata["uid"]); userID != "" {
			return userID
		}
		if email := strings.TrimSpace(session.Metadata["email"]); email != "" {
			return email
		}
	}
	return session.ID
}

func buildBudgetStatus(budget SectionBudget, tokens int, threshold float64) SectionBudgetStatus {
	if threshold == 0 {
		threshold = defaultThreshold
	}
	status := SectionBudgetStatus{Limit: budget.Limit, Threshold: threshold, Tokens: tokens}
	if budget.Limit > 0 {
		status.ShouldCompress = float64(tokens) > float64(budget.Limit)*threshold
	}
	return status
}

func estimateTokensForString(value string) int {
	if value == "" {
		return 0
	}
	return len([]rune(value)) / 4
}

func estimateTokensForStruct(value any) int {
	raw, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return estimateTokensForString(string(raw))
}

func computeEnvelopeHash(window ports.ContextWindow, userRef string) string {
	payload := map[string]any{
		"session_id": window.SessionID,
		"user_ref":   userRef,
		"system":     window.SystemPrompt,
		"static":     window.Static,
		"dynamic":    window.Dynamic,
		"meta":       window.Meta,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
