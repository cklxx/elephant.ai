//go:build ignore
// Package evolution provides self-improving agent capabilities.
// An agent that can analyze its own performance, learn from feedback,
// and iteratively optimize its behavior over time.
package evolution

import (
	"context"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

// EvolutionRecord tracks a single learning event or adaptation.
type EvolutionRecord struct {
	ID            string                 `json:"id"`
	Timestamp     time.Time              `json:"timestamp"`
	Type          EvolutionType          `json:"type"`
	Trigger       EvolutionTrigger       `json:"trigger"`
	Before        map[string]interface{} `json:"before"`
	After         map[string]interface{} `json:"after"`
	Reasoning     string                 `json:"reasoning"`
	PerformanceDelta float64             `json:"performance_delta"`
	SessionID     string                 `json:"session_id"`
	Accepted      bool                   `json:"accepted"`
}

// EvolutionType categorizes what kind of adaptation occurred.
type EvolutionType string

const (
	EvolutionTypePrompt      EvolutionType = "prompt_optimization"
	EvolutionTypeStrategy    EvolutionType = "strategy_adjustment"
	EvolutionTypeToolUse     EvolutionType = "tool_use_improvement"
	EvolutionTypeContext     EvolutionType = "context_management"
	EvolutionTypePersona     EvolutionType = "persona_refine"
	EvolutionTypeReflection  EvolutionType = "self_reflection"
)

// EvolutionTrigger indicates what caused the evolution.
type EvolutionTrigger string

const (
	TriggerUserFeedback   EvolutionTrigger = "user_feedback"
	TriggerPerformanceDrop EvolutionTrigger = "performance_drop"
	TriggerPatternMatch   EvolutionTrigger = "pattern_match"
	TriggerSelfReflection EvolutionTrigger = "self_reflection"
	TriggerErrorRecovery  EvolutionTrigger = "error_recovery"
	TriggerGoalCompletion EvolutionTrigger = "goal_completion"
)

// PerformanceMetrics captures execution quality indicators.
type PerformanceMetrics struct {
	TaskID          string        `json:"task_id"`
	SessionID       string        `json:"session_id"`
	StartedAt       time.Time     `json:"started_at"`
	CompletedAt     time.Time     `json:"completed_at"`
	Duration        time.Duration `json:"duration"`
	Iterations      int           `json:"iterations"`
	TokenUsage      int           `json:"token_usage"`
	ToolCalls       int           `json:"tool_calls"`
	Errors          int           `json:"errors"`
	UserRating      int           `json:"user_rating"`
	Success         bool          `json:"success"`
	StopReason      string        `json:"stop_reason"`
	
	// Quality scores (0-1)
	RelevanceScore   float64 `json:"relevance_score"`
	EfficiencyScore  float64 `json:"efficiency_score"`
	CorrectnessScore float64 `json:"correctness_score"`
	
	// Raw data for analysis
	Messages        []ports.Message `json:"messages"`
	FinalAnswer     string          `json:"final_answer"`
	UserFeedback    string          `json:"user_feedback"`
}

// LearningPattern represents a discovered reusable insight.
type LearningPattern struct {
	ID              string                 `json:"id"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	PatternType     string                 `json:"pattern_type"`
	Description     string                 `json:"description"`
	Condition       PatternCondition       `json:"condition"`
	Action          PatternAction          `json:"action"`
	UsageCount      int                    `json:"usage_count"`
	SuccessRate     float64                `json:"success_rate"`
	LastUsedAt      *time.Time             `json:"last_used_at,omitempty"`
	Confidence      float64                `json:"confidence"`
	Tags            []string               `json:"tags"`
}

// PatternCondition defines when a pattern applies.
type PatternCondition struct {
	TaskKeywords    []string `json:"task_keywords,omitempty"`
	ErrorPatterns   []string `json:"error_patterns,omitempty"`
	ToolSequence    []string `json:"tool_sequence,omitempty"`
	MinIterations   int      `json:"min_iterations,omitempty"`
	MaxIterations   int      `json:"max_iterations,omitempty"`
	CustomMatcher   string   `json:"custom_matcher,omitempty"`
}

// PatternAction defines what to do when pattern matches.
type PatternAction struct {
	PromptAugmentation string   `json:"prompt_augmentation,omitempty"`
	PreferredTools     []string `json:"preferred_tools,omitempty"`
	StrategyHint       string   `json:"strategy_hint,omitempty"`
	ContextPriority    []string `json:"context_priority,omitempty"`
}

// Feedback captures user or system feedback on agent performance.
type Feedback struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	TaskID      string    `json:"task_id"`
	SessionID   string    `json:"session_id"`
	Source      FeedbackSource `json:"source"`
	Rating      int       `json:"rating"` // 1-5 or -1, 0, 1
	Comment     string    `json:"comment"`
	Category    string    `json:"category"`
	Specifics   map[string]string `json:"specifics"`
}

type FeedbackSource string

const (
	FeedbackSourceUser     FeedbackSource = "user"
	FeedbackSourceSystem   FeedbackSource = "system"
	FeedbackSourceSelf     FeedbackSource = "self"
	FeedbackSourceExternal FeedbackSource = "external"
)

// EvolutionConfig controls self-improvement behavior.
type EvolutionConfig struct {
	Enabled              bool          `json:"enabled"`
	AutoApplyPromptChanges bool        `json:"auto_apply_prompt_changes"`
	AutoApplyStrategyChanges bool      `json:"auto_apply_strategy_changes"`
	ReflectionInterval     int         `json:"reflection_interval"` // tasks between reflections
	MinFeedbackForPattern  int         `json:"min_feedback_for_pattern"`
	ConfidenceThreshold    float64     `json:"confidence_threshold"`
	MaxPatterns            int         `json:"max_patterns"`
	EvolutionHistoryLimit  int         `json:"evolution_history_limit"`
}

// DefaultEvolutionConfig returns sensible defaults.
func DefaultEvolutionConfig() EvolutionConfig {
	return EvolutionConfig{
		Enabled:                    true,
		AutoApplyPromptChanges:     false, // require approval by default
		AutoApplyStrategyChanges:   true,
		ReflectionInterval:         10,
		MinFeedbackForPattern:      3,
		ConfidenceThreshold:        0.7,
		MaxPatterns:                50,
		EvolutionHistoryLimit:      100,
	}
}

// EvolutionEngine defines the interface for evolution capabilities
type EvolutionEngine interface {
	RecordPerformance(ctx context.Context, metrics PerformanceMetrics) error
	RecordFeedback(ctx context.Context, feedback Feedback) error
	AnalyzeTrends(ctx context.Context, window time.Duration) (*TrendAnalysis, error)
	GenerateImprovement(ctx context.Context, analysis *TrendAnalysis) (*ImprovementProposal, error)
	ApplyImprovement(ctx context.Context, proposal *ImprovementProposal) (*EvolutionRecord, error)
	GetPatterns(ctx context.Context, contextHint string) ([]LearningPattern, error)
	Reflect(ctx context.Context) (*ReflectionResult, error)
}

// TrendAnalysis aggregates performance insights.
type TrendAnalysis struct {
	WindowStart     time.Time       `json:"window_start"`
	WindowEnd       time.Time       `json:"window_end"`
	TaskCount       int             `json:"task_count"`
	SuccessRate     float64         `json:"success_rate"`
	AvgIterations   float64         `json:"avg_iterations"`
	AvgTokenUsage   float64         `json:"avg_token_usage"`
	CommonErrors    []ErrorPattern  `json:"common_errors"`
	Inefficiencies  []string        `json:"inefficiencies"`
	Strengths       []string        `json:"strengths"`
}

// ErrorPattern captures recurring error signatures.
type ErrorPattern struct {
	Pattern     string  `json:"pattern"`
	Count       int     `json:"count"`
	ToolName    string  `json:"tool_name,omitempty"`
	RecoveryRate float64 `json:"recovery_rate"`
}

// ImprovementProposal suggests a specific optimization.
type ImprovementProposal struct {
	ID           string            `json:"id"`
	Type         EvolutionType     `json:"type"`
	Description  string            `json:"description"`
	ExpectedGain float64           `json:"expected_gain"`
	Changes      map[string]Change `json:"changes"`
	RiskLevel    string            `json:"risk_level"` // low, medium, high
	RollbackPlan string            `json:"rollback_plan"`
}

// Change describes a specific modification.
type Change struct {
	Path    string `json:"path"`
	Before  string `json:"before"`
	After   string `json:"after"`
	Reason  string `json:"reason"`
}

// ReflectionResult captures deep self-analysis output.
type ReflectionResult struct {
	Timestamp       time.Time       `json:"timestamp"`
	Insights        []string        `json:"insights"`
	ProposedGoals   []string        `json:"proposed_goals"`
	PatternIdeas    []PatternIdea   `json:"pattern_ideas"`
	ConfidenceDelta map[string]float64 `json:"confidence_delta"`
}

// PatternIdea is a candidate pattern from reflection.
type PatternIdea struct {
	Description string   `json:"description"`
	TriggerHint string   `json:"trigger_hint"`
	ActionHint  string   `json:"action_hint"`
	Confidence  float64  `json:"confidence"`
}

// Analyzer types
type PerformanceAnalysis struct {
	RecordID              string                 `json:"record_id"`
	AnalyzedAt            time.Time              `json:"analyzed_at"`
	TaskType              string                 `json:"task_type"`
	Duration              time.Duration          `json:"duration"`
	IterationCount        int                    `json:"iteration_count"`
	TokenUsage            int                    `json:"token_usage"`
	ToolCalls             int                    `json:"tool_calls"`
	Success               bool                   `json:"success"`
	Errors                []string               `json:"errors"`
	EfficiencyScore       float64                `json:"efficiency_score"`
	ImprovementSuggestions []ImprovementSuggestion `json:"improvement_suggestions"`
	SuccessPatterns       []string               `json:"success_patterns"`
	FailurePatterns       []string               `json:"failure_patterns"`
}

type ImprovementSuggestion struct {
	Category    string `json:"category"`
	Priority    string `json:"priority"`
	Description string `json:"description"`
	Action      string `json:"action"`
}

type PerformanceBaseline struct {
	ID                   string        `json:"id"`
	TaskType             string        `json:"task_type"`
	CreatedAt            time.Time     `json:"created_at"`
	AvgDuration          time.Duration `json:"avg_duration"`
	AvgTokenUsage        int           `json:"avg_token_usage"`
	AvgIterations        float64       `json:"avg_iterations"`
	AvgEfficiencyScore   float64       `json:"avg_efficiency_score"`
}

type PerformanceDelta struct {
	TaskType         string        `json:"task_type"`
	ComparedAt       time.Time     `json:"compared_at"`
	BaselineID       string        `json:"baseline_id"`
	DurationDelta    float64       `json:"duration_delta"`
	TokenDelta       float64       `json:"token_delta"`
	IterationDelta   float64       `json:"iteration_delta"`
	EfficiencyDelta  float64       `json:"efficiency_delta"`
	Trend            string        `json:"trend"`
}

type PerformanceTrend struct {
	TaskType         string        `json:"task_type"`
	WindowSize       int           `json:"window_size"`
	StartTime        time.Time     `json:"start_time"`
	EndTime          time.Time     `json:"end_time"`
	AvgDurationMs    float64       `json:"avg_duration_ms"`
	AvgTokenUsage    int           `json:"avg_token_usage"`
	AvgIterations    float64       `json:"avg_iterations"`
	Direction        string        `json:"direction"`
	EfficiencyChange float64       `json:"efficiency_change"`
}

// EvolutionHistory stores execution history for analysis
type EvolutionHistory struct {
	records []ExecutionRecord
	mu      sync.RWMutex
}

func NewEvolutionHistory() *EvolutionHistory {
	return &EvolutionHistory{
		records: make([]ExecutionRecord, 0),
	}
}

func (h *EvolutionHistory) AddRecord(record ExecutionRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, record)
}

func (h *EvolutionHistory) GetRecentRecords(n int) []ExecutionRecord {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if n >= len(h.records) {
		return append([]ExecutionRecord(nil), h.records...)
	}
	return append([]ExecutionRecord(nil), h.records[len(h.records)-n:]...)
}

// Constants for task types and trends
const (
	TaskTypeSimple   = "simple"
	TaskTypeComplex  = "complex"
	TaskTypeResearch = "research"
	TaskTypeCode     = "code"
	
	TrendImproving = "improving"
	TrendStable    = "stable"
	TrendDegrading = "degrading"
	
	SuggestionCategoryPlanning   = "planning"
	SuggestionCategoryContext    = "context"
	SuggestionCategoryTools      = "tools"
	SuggestionCategoryExecution  = "execution"
	SuggestionCategoryPrompt     = "prompt"
	
	PriorityHigh   = "high"
	PriorityMedium = "medium"
	PriorityLow    = "low"
)

// Optimizer types
type LLMClient interface {
	Generate(ctx context.Context, prompt string, options ...any) (string, error)
}

type PromptOptimization struct {
	OriginalPrompt   string   `json:"original_prompt"`
	OptimizedPrompt  string   `json:"optimized_prompt"`
	Changes          []string `json:"changes"`
	ExpectedImprovement float64 `json:"expected_improvement"`
	Confidence       float64  `json:"confidence"`
}

type OptimizationStrategy string

const (
	OptimizationStrategyConservative OptimizationStrategy = "conservative"
	OptimizationStrategyIterative    OptimizationStrategy = "iterative"
	OptimizationStrategyAggressive   OptimizationStrategy = "aggressive"
)

type PromptChangeType string

const (
	PromptChangeMinor      PromptChangeType = "minor"
	PromptChangeRefinement PromptChangeType = "refinement"
	PromptChangeMajor      PromptChangeType = "major"
)

// Engine types
type EvolutionEvent struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

type LearningStorage interface {
	SavePattern(ctx context.Context, pattern *LearnedPattern) error
	GetPatterns(ctx context.Context, domain string) ([]*LearnedPattern, error)
	UpdatePatternSuccess(ctx context.Context, patternID string, success bool) error
}

// InMemoryLearningStorage is a simple in-memory implementation
type InMemoryLearningStorage struct {
	patterns map[string]*LearnedPattern
	mu       sync.RWMutex
}

func NewInMemoryLearningStorage() *InMemoryLearningStorage {
	return &InMemoryLearningStorage{
		patterns: make(map[string]*LearnedPattern),
	}
}

func (s *InMemoryLearningStorage) SavePattern(ctx context.Context, pattern *LearnedPattern) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.patterns[pattern.ID] = pattern
	return nil
}

func (s *InMemoryLearningStorage) GetPatterns(ctx context.Context, domain string) ([]*LearnedPattern, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*LearnedPattern
	for _, p := range s.patterns {
		if domain == "" || p.Domain == domain {
			result = append(result, p)
		}
	}
	return result, nil
}

func (s *InMemoryLearningStorage) UpdatePatternSuccess(ctx context.Context, patternID string, success bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p, ok := s.patterns[patternID]; ok {
		if success {
			p.SuccessCount++
		} else {
			p.FailureCount++
		}
		p.UpdatedAt = time.Now()
	}
	return nil
}

// Feedback types
type FeedbackEntry struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	RunID       string    `json:"run_id"`
	Timestamp   time.Time `json:"timestamp"`
	Rating      int       `json:"rating"`
	Category    string    `json:"category"`
	Comment     string    `json:"comment"`
	Source      string    `json:"source"`
}

// Note: EvolutionStore interface is defined in memory.go
// Note: BaseAgentExecutor and Logger interfaces are defined in agent.go

// Logger is the interface for logging in evolution package.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
}

// AppliedImprovement represents an improvement that has been applied to the agent.
type AppliedImprovement struct {
	ID          string    `json:"id"`
	AppliedAt   time.Time `json:"applied_at"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Changes     []Change  `json:"changes"`
}

// TaskExecution represents a single task execution record.
type TaskExecution struct {
	ID          string        `json:"id"`
	SessionID   string        `json:"session_id"`
	Task        string        `json:"task"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	Iterations  int           `json:"iterations"`
	TokenUsage  int           `json:"token_usage"`
	ToolCalls   int           `json:"tool_calls"`
	Error       string        `json:"error,omitempty"`
}

// FeedbackItem represents feedback on a task execution.
type FeedbackItem struct {
	ID          string    `json:"id"`
	ExecutionID string    `json:"execution_id"`
	SessionID   string    `json:"session_id"`
	Rating      int       `json:"rating"`
	Comment     string    `json:"comment,omitempty"`
	Category    string    `json:"category,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// LearningEntry represents a single learning entry.
type LearningEntry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Source    string    `json:"source"`
	Confidence float64  `json:"confidence"`
}

// Note: ExecutionRecord is defined in agent.go
// Note: LearnedPattern is defined in memory.go

// Agent is an alias to the agent port interface
type Agent = agent.Agent

// EvolutionReport represents a report of evolution activities
type EvolutionReport struct {
	Timestamp    time.Time         `json:"timestamp"`
	Period       time.Duration     `json:"period"`
	Improvements []string          `json:"improvements"`
	Metrics      map[string]float64 `json:"metrics"`
}

// ExplicitFeedbackRequest represents a request for explicit user feedback
type ExplicitFeedbackRequest struct {
	SessionID   string `json:"session_id"`
	TaskID      string `json:"task_id"`
	Question    string `json:"question"`
	Category    string `json:"category"`
	RequestedAt time.Time `json:"requested_at"`
}

// ImplicitSignal represents implicit feedback signals
type ImplicitSignal struct {
	Type      string    `json:"type"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	Context   map[string]string `json:"context"`
}

// FeedbackSummary summarizes feedback over a period
type FeedbackSummary struct {
	Period       time.Duration `json:"period"`
	TotalCount   int           `json:"total_count"`
	AverageRating float64      `json:"average_rating"`
	Categories   map[string]int `json:"categories"`
}