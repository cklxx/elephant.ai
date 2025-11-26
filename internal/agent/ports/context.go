package ports

import (
	"context"
	"time"
)

// ContextManager handles layered context orchestration across static, dynamic and
// meta layers.
type ContextManager interface {
	// EstimateTokens estimates token count for messages.
	EstimateTokens(messages []Message) int

	// Compress reduces message size when limit approached.
	Compress(messages []Message, targetTokens int) ([]Message, error)

	// ShouldCompress checks if compression needed.
	ShouldCompress(messages []Message, limit int) bool

	// Preload ensures the manager has cached static context/configuration before
	// first use.
	Preload(ctx context.Context) error

	// BuildWindow composes the full context window for the given session and
	// configuration.
	BuildWindow(ctx context.Context, session *Session, cfg ContextWindowConfig) (ContextWindow, error)

	// RecordTurn writes the supplied turn record to the dynamic state store so
	// that UI/API consumers can replay the session.
	RecordTurn(ctx context.Context, record ContextTurnRecord) error
}

// SessionContextKey is the shared context key for storing session IDs across packages.
// This ensures consistent session ID propagation from server layer to agent layer.
type SessionContextKey struct{}

// ContextWindowConfig drives context composition behaviour.
type ContextWindowConfig struct {
	TokenLimit           int
	Budgets              ContextBudgets
	PersonaKey           string
	GoalKey              string
	WorldKey             string
	ToolPreset           string
	EnvironmentSummary   string
	ExpectedEnvelopeHash string `json:"expected_envelope_hash" yaml:"expected_envelope_hash"`
}

// ContextBudgets defines per-section token limits and an optional threshold.
// If limits are zero, defaults configured in the context manager are used.
type ContextBudgets struct {
	Threshold float64 `json:"threshold" yaml:"threshold"`
	System    int     `json:"system" yaml:"system"`
	Static    int     `json:"static" yaml:"static"`
	Dynamic   int     `json:"dynamic" yaml:"dynamic"`
	Meta      int     `json:"meta" yaml:"meta"`
}

// ContextWindow exposes the layered context returned by the manager.
type ContextWindow struct {
	SessionID    string         `json:"session_id"`
	Messages     []Message      `json:"messages"`
	SystemPrompt string         `json:"system_prompt"`
	Static       StaticContext  `json:"static"`
	Dynamic      DynamicContext `json:"dynamic"`
	Meta         MetaContext    `json:"meta"`

	// EnvelopeHash pins the exact combination of static/dynamic/meta slices used
	// to build this window so caches can detect churn.
	EnvelopeHash string `json:"envelope_hash,omitempty"`

	// Budgets captures per-section token utilization and whether compression is
	// advised. Keys align to section names used by the context manager (system,
	// static, dynamic, meta).
	Budgets []SectionBudgetStatus `json:"budgets,omitempty"`

	// TokensBySection exposes raw token estimates for observability and UI display.
	TokensBySection map[string]int `json:"tokens_by_section,omitempty"`
}

// StaticContext captures persona, goals, rules and knowledge packs.
type StaticContext struct {
	Persona            PersonaProfile       `json:"persona"`
	Goal               GoalProfile          `json:"goal"`
	Policies           []PolicyRule         `json:"policies"`
	Knowledge          []KnowledgeReference `json:"knowledge"`
	Tools              []string             `json:"tools"`
	World              WorldProfile         `json:"world"`
	EnvironmentSummary string               `json:"environment_summary,omitempty"`
	Version            string               `json:"version,omitempty"`
}

// DynamicContext contains plans, beliefs and world-state diffs.
type DynamicContext struct {
	TurnID            int              `json:"turn_id"`
	LLMTurnSeq        int              `json:"llm_turn_seq"`
	Plans             []PlanNode       `json:"plans"`
	Beliefs           []Belief         `json:"beliefs"`
	WorldState        map[string]any   `json:"world_state"`
	Feedback          []FeedbackSignal `json:"feedback"`
	SnapshotTimestamp time.Time        `json:"snapshot_timestamp"`
}

// MetaContext tracks long-horizon memories and persona bookkeeping.
type MetaContext struct {
	Memories        []MemoryFragment `json:"memories" yaml:"memories"`
	Recommendations []string         `json:"recommendations" yaml:"recommendations"`
	PersonaVersion  string           `json:"persona_version" yaml:"persona_version"`
}

// PersonaProfile models persona level instructions.
type PersonaProfile struct {
	ID            string `json:"id" yaml:"id"`
	Tone          string `json:"tone" yaml:"tone"`
	RiskProfile   string `json:"risk_profile" yaml:"risk_profile"`
	DecisionStyle string `json:"decision_style" yaml:"decision_style"`
	Voice         string `json:"voice" yaml:"voice"`
}

// GoalProfile enumerates long and mid-term goals.
type GoalProfile struct {
	ID             string   `json:"id" yaml:"id"`
	LongTerm       []string `json:"long_term" yaml:"long_term"`
	MidTerm        []string `json:"mid_term" yaml:"mid_term"`
	SuccessMetrics []string `json:"success_metrics" yaml:"success_metrics"`
}

// PolicyRule contains explicit guardrails/preference statements.
type PolicyRule struct {
	ID              string   `json:"id" yaml:"id"`
	HardConstraints []string `json:"hard_constraints" yaml:"hard_constraints"`
	SoftPreferences []string `json:"soft_preferences" yaml:"soft_preferences"`
	RewardHooks     []string `json:"reward_hooks" yaml:"reward_hooks"`
}

// KnowledgeReference references SOP or RAG collections.
type KnowledgeReference struct {
	ID             string   `json:"id" yaml:"id"`
	Description    string   `json:"description" yaml:"description"`
	SOPRefs        []string `json:"sop_refs" yaml:"sop_refs"`
	RAGCollections []string `json:"rag_collections" yaml:"rag_collections"`
	MemoryKeys     []string `json:"memory_keys" yaml:"memory_keys"`
}

// WorldProfile enumerates runtime environment capabilities and limits.
type WorldProfile struct {
	ID           string   `json:"id" yaml:"id"`
	Environment  string   `json:"environment" yaml:"environment"`
	Capabilities []string `json:"capabilities" yaml:"capabilities"`
	Limits       []string `json:"limits" yaml:"limits"`
	CostModel    []string `json:"cost_model" yaml:"cost_model"`
}

// PlanNode encodes nested plan trees.
type PlanNode struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Status      string     `json:"status"`
	Children    []PlanNode `json:"children"`
	Description string     `json:"description,omitempty"`
}

// Belief captures assumptions or facts.
type Belief struct {
	Statement  string  `json:"statement"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source,omitempty"`
}

// FeedbackSignal carries lightweight scoring signals.
type FeedbackSignal struct {
	Kind      string    `json:"kind"`
	Message   string    `json:"message"`
	Value     float64   `json:"value"`
	CreatedAt time.Time `json:"created_at"`
}

// SectionBudgetStatus reports current token usage against a section budget.
type SectionBudgetStatus struct {
	Section        string  `json:"section"`
	Limit          int     `json:"limit"`
	Threshold      float64 `json:"threshold"`
	Tokens         int     `json:"tokens"`
	ShouldCompress bool    `json:"should_compress"`
}

// MemoryFragment references retained memories.
type MemoryFragment struct {
	Key       string    `json:"key" yaml:"key"`
	Content   string    `json:"content" yaml:"content"`
	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	Source    string    `json:"source" yaml:"source"`
}

// ContextTurnRecord is persisted each time the LLM is invoked.
type ContextTurnRecord struct {
	SessionID     string               `json:"session_id"`
	TurnID        int                  `json:"turn_id"`
	LLMTurnSeq    int                  `json:"llm_turn_seq"`
	Timestamp     time.Time            `json:"timestamp"`
	Summary       string               `json:"summary"`
	Plans         []PlanNode           `json:"plans"`
	Beliefs       []Belief             `json:"beliefs"`
	World         map[string]any       `json:"world_state"`
	Diff          map[string]any       `json:"diff"`
	Messages      []Message            `json:"messages"`
	Feedback      []FeedbackSignal     `json:"feedback"`
	KnowledgeRefs []KnowledgeReference `json:"knowledge_refs"`
}
