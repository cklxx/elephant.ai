package agent

import (
	"context"
	"time"

	core "alex/internal/agent/ports"
	"alex/internal/agent/ports/storage"
)

// ContextManager handles layered context orchestration across static, dynamic and
// meta layers.
type ContextManager interface {
	// EstimateTokens estimates token count for messages.
	EstimateTokens(messages []core.Message) int

	// Compress reduces message size when limit approached.
	Compress(messages []core.Message, targetTokens int) ([]core.Message, error)

	// AutoCompact applies compression automatically when the configured
	// threshold is exceeded. It returns the possibly compacted slice and a flag
	// indicating whether compaction occurred.
	AutoCompact(messages []core.Message, limit int) ([]core.Message, bool)

	// ShouldCompress checks if compression needed.
	ShouldCompress(messages []core.Message, limit int) bool

	// Preload ensures the manager has cached static context/configuration before
	// first use.
	Preload(ctx context.Context) error

	// BuildWindow composes the full context window for the given session and
	// configuration.
	BuildWindow(ctx context.Context, session *storage.Session, cfg ContextWindowConfig) (ContextWindow, error)

	// RecordTurn writes the supplied turn record to the dynamic state store so
	// that UI/API consumers can replay the session.
	RecordTurn(ctx context.Context, record ContextTurnRecord) error
}

// SessionContextKey is the shared context key for storing session IDs across packages.
// This ensures consistent session ID propagation from server layer to agent layer.
type SessionContextKey struct{}

// ContextWindowConfig drives context composition behaviour.
type ContextWindowConfig struct {
	TokenLimit         int
	PersonaKey         string
	GoalKey            string
	WorldKey           string
	ToolMode           string
	ToolPreset         string
	EnvironmentSummary string
}

// ContextWindow exposes the layered context returned by the manager.
type ContextWindow struct {
	SessionID    string         `json:"session_id"`
	Messages     []core.Message `json:"messages"`
	SystemPrompt string         `json:"system_prompt"`
	Static       StaticContext  `json:"static"`
	Dynamic      DynamicContext `json:"dynamic"`
	Meta         MetaContext    `json:"meta"`
}

// ContextWindowPreview bundles the constructed window with metadata useful for
// debugging and visualization.
type ContextWindowPreview struct {
	Window        ContextWindow `json:"window"`
	TokenEstimate int           `json:"token_estimate"`
	TokenLimit    int           `json:"token_limit"`
	PersonaKey    string        `json:"persona_key,omitempty"`
	ToolMode      string        `json:"tool_mode,omitempty"`
	ToolPreset    string        `json:"tool_preset,omitempty"`
}

// StaticContext captures persona, goals, rules and knowledge packs.
type StaticContext struct {
	Persona            PersonaProfile       `json:"persona"`
	Goal               GoalProfile          `json:"goal"`
	Policies           []PolicyRule         `json:"policies"`
	Knowledge          []KnowledgeReference `json:"knowledge"`
	Tools              []string             `json:"tools"`
	World              WorldProfile         `json:"world"`
	UserPersona        *core.UserPersonaProfile `json:"user_persona,omitempty"`
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
	Memories        []MemoryFragment `json:"memories"`
	Recommendations []string         `json:"recommendations"`
	PersonaVersion  string           `json:"persona_version"`
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

	// ResolvedSOPContent holds the resolved markdown content for each SOP ref.
	// Populated at runtime by SOPResolver; never read from config.
	ResolvedSOPContent map[string]string `json:"resolved_sop_content,omitempty" yaml:"-"`
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

// MemoryFragment references retained memories.
type MemoryFragment struct {
	Key       string    `json:"key"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	Source    string    `json:"source"`
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
	Messages      []core.Message       `json:"messages"`
	Feedback      []FeedbackSignal     `json:"feedback"`
	KnowledgeRefs []KnowledgeReference `json:"knowledge_refs"`
}
