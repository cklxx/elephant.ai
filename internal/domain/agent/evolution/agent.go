//go:build ignore
package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

// BaseAgentExecutor is the underlying agent that performs actual task execution.
type BaseAgentExecutor interface {
	Execute(ctx context.Context, task string, opts ExecuteOptions) (*ExecuteResult, error)
}

// ExecuteOptions contains options for task execution.
type ExecuteOptions struct {
	SessionID       string
	SystemPrompt    string
	MaxIterations   int
	EnableStreaming bool
	Metadata        map[string]string
}

// ExecuteResult contains the outcome of task execution.
type ExecuteResult struct {
	Answer      string
	Iterations  int
	TokensUsed  int
	Duration    time.Duration
	StopReason  string
	ToolCalls   []ports.ToolCall
	Messages    []ports.Message
	SessionID   string
	Attachments map[string]ports.Attachment
}

// SelfEvolvingAgent is a wrapper that adds self-evolution capabilities to a base agent.
type SelfEvolvingAgent struct {
	baseAgent        BaseAgentExecutor
	evolutionEngine  *EvolutionEngine
	logger           Logger
	config           EvolvingAgentConfig
	sessionLearnings map[string]*SessionLearning
	evolutionHistory []SessionEvolutionRecord
	mu               sync.RWMutex
}

// EvolvingAgentConfig configures the self-evolution behavior.
type EvolvingAgentConfig struct {
	// Evolution triggers
	EnableAutoEvolution   bool          // Whether to automatically evolve
	MinSamplesForEvolve   int           // Minimum samples needed before evolution
	EvolutionCooldown     time.Duration // Minimum time between evolutions

	// Feedback collection
	EnableExplicitFeedback bool // Allow users to provide explicit feedback
	EnableImplicitFeedback bool // Learn from execution patterns

	// Learning scope
	LearnFromSuccess      bool // Learn what works well
	LearnFromFailure      bool // Learn from mistakes
	CrossSessionLearning  bool // Share learnings across sessions

	// Safety limits
	MaxEvolutionsPerDay   int     // Limit evolution rate
	MinConfidenceForApply float64 // Minimum confidence to apply changes
}

// DefaultEvolvingAgentConfig returns sensible defaults.
func DefaultEvolvingAgentConfig() EvolvingAgentConfig {
	return EvolvingAgentConfig{
		EnableAutoEvolution:    true,
		MinSamplesForEvolve:    3,
		EvolutionCooldown:      time.Hour,
		EnableExplicitFeedback: true,
		EnableImplicitFeedback: true,
		LearnFromSuccess:       true,
		LearnFromFailure:       true,
		CrossSessionLearning:   true,
		MaxEvolutionsPerDay:    10,
		MinConfidenceForApply:  0.7,
	}
}

// NewSelfEvolvingAgent creates a new self-evolving agent wrapper.
func NewSelfEvolvingAgent(
	baseAgent BaseAgentExecutor,
	evolutionEngine *EvolutionEngine,
	logger Logger,
	config EvolvingAgentConfig,
) *SelfEvolvingAgent {
	if logger == nil {
		logger = &noopLogger{}
	}

	return &SelfEvolvingAgent{
		baseAgent:        baseAgent,
		evolutionEngine:  evolutionEngine,
		logger:           logger,
		config:           config,
		sessionLearnings: make(map[string]*SessionLearning),
		evolutionHistory: make([]SessionEvolutionRecord, 0),
	}
}

// Execute runs a task with self-evolution capabilities.
func (a *SelfEvolvingAgent) Execute(ctx context.Context, task string, opts ExecuteOptions) (*ExecuteResult, error) {
	// Get or create session learning state
	learning := a.getOrCreateSessionLearning(opts.SessionID)

	// Enhance system prompt with evolution context
	enhancedPrompt := a.enhanceSystemPrompt(opts.SystemPrompt, learning)
	opts.SystemPrompt = enhancedPrompt

	// Execute with the base agent
	startTime := time.Now()
	result, err := a.baseAgent.Execute(ctx, task, opts)
	duration := time.Since(startTime)

	if err != nil {
		a.logger.Error("Base agent execution failed", "error", err, "session", opts.SessionID)
		return nil, err
	}

	// Record execution for learning
	execution := a.recordExecution(learning, task, result, duration)

	// Trigger implicit learning analysis
	if a.config.EnableImplicitFeedback {
		a.analyzeExecution(ctx, execution, learning)
	}

	// Check if we should trigger evolution
	if a.shouldTriggerEvolution(learning) {
		if evolveErr := a.triggerEvolution(ctx, opts.SessionID); evolveErr != nil {
			a.logger.Warn("Evolution failed", "error", evolveErr)
		}
	}

	return result, nil
}

// SubmitFeedback allows explicit user feedback on execution results.
func (a *SelfEvolvingAgent) SubmitFeedback(ctx context.Context, sessionID string, feedback ExecutionFeedback) error {
	learning := a.getOrCreateSessionLearning(sessionID)

	feedbackEvent := FeedbackEvent{
		ID:          generateFeedbackID(),
		SessionID:   sessionID,
		RunID:       feedback.RunID,
		Timestamp:   time.Now(),
		Rating:      feedback.Rating,
		Category:    feedback.Category,
		Comment:     feedback.Comment,
		Suggested:   feedback.SuggestedImprovement,
		AnnotatedBy: feedback.Source,
	}

	// Store feedback
	learning.FeedbackHistory = append(learning.FeedbackHistory, feedbackEvent)

	// Trigger learning immediately on explicit feedback
	if a.config.EnableExplicitFeedback {
		record := LearningRecord{
			ID:            generateLearningID(),
			Timestamp:     time.Now(),
			SessionID:     sessionID,
			FeedbackID:    feedbackEvent.ID,
			Type:          LearningTypeFeedback,
			Trigger:       LearningTriggerUserFeedback,
			SourcePattern: "user_feedback",
			Insight: Insight{
				Description:   feedback.Comment,
				Confidence:    float64(feedback.Rating) / 5.0,
				Category:      feedback.Category,
				Applicable:    true,
				CreatedAt:     time.Now(),
			},
		}

		if err := a.evolutionEngine.LearningStore().Store(ctx, record); err != nil {
			a.logger.Error("Failed to store learning from feedback", "error", err)
			return err
		}

		a.logger.Info("Stored learning from user feedback",
			"session", sessionID,
			"rating", feedback.Rating,
			"category", feedback.Category)
	}

	return nil
}

// GetEvolutionStatus returns current evolution status for a session.
func (a *SelfEvolvingAgent) GetEvolutionStatus(sessionID string) EvolutionStatus {
	learning := a.getOrCreateSessionLearning(sessionID)

	return EvolutionStatus{
		SessionID:           sessionID,
		TotalExecutions:     learning.ExecutionCount,
		TotalFeedbacks:      len(learning.FeedbackHistory),
		LearningsApplied:    learning.AppliedLearnings,
		CurrentGeneration:   learning.Generation,
		LastEvolutionTime:   learning.LastEvolutionTime,
		PendingImprovements: learning.PendingImprovements,
		EvolutionHistory:    a.evolutionHistory,
	}
}

// GetAppliedImprovements returns improvements applied to the agent.
func (a *SelfEvolvingAgent) GetAppliedImprovements() []AppliedImprovement {
	return a.evolutionEngine.GetAppliedImprovements()
}

// Internal helper methods

func (a *SelfEvolvingAgent) getOrCreateSessionLearning(sessionID string) *SessionLearning {
	if learning, exists := a.sessionLearnings[sessionID]; exists {
		return learning
	}

	learning := &SessionLearning{
		SessionID:             sessionID,
		Generation:            1,
		ExecutionCount:        0,
		ExecutionHistory:      make([]ExecutionRecord, 0),
		FeedbackHistory:       make([]FeedbackEvent, 0),
		AppliedLearnings:      0,
		PendingImprovements:   make([]Improvement, 0),
		CreatedAt:             time.Now(),
		LastEvolutionTime:     time.Time{},
	}

	a.sessionLearnings[sessionID] = learning
	return learning
}

func (a *SelfEvolvingAgent) enhanceSystemPrompt(basePrompt string, learning *SessionLearning) string {
	if learning == nil || len(learning.PendingImprovements) == 0 {
		return basePrompt
	}

	// Add evolution context to system prompt
	enhancement := "\n\n=== Self-Evolution Context ===\n"
	enhancement += fmt.Sprintf("Current Generation: %d\n", learning.Generation)
	enhancement += fmt.Sprintf("Applied Learnings: %d\n", learning.AppliedLearnings)

	if len(learning.PendingImprovements) > 0 {
		enhancement += "\nActive Improvements:\n"
		for i, imp := range learning.PendingImprovements {
			if i >= 3 { // Limit to top 3
				enhancement += fmt.Sprintf("- ... and %d more\n", len(learning.PendingImprovements)-i)
				break
			}
			enhancement += fmt.Sprintf("- %s: %s\n", imp.Category, imp.Description)
		}
	}

	enhancement += "\n=== End Evolution Context ==="

	return basePrompt + enhancement
}

func (a *SelfEvolvingAgent) recordExecution(learning *SessionLearning, task string, result *ExecuteResult, duration time.Duration) ExecutionRecord {
	record := ExecutionRecord{
		ID:          generateExecutionID(),
		Timestamp:   time.Now(),
		Task:        task,
		Result:      result.Answer,
		Iterations:  result.Iterations,
		TokensUsed:  result.TokensUsed,
		Duration:    duration,
		Success:     result.StopReason == "complete" || result.StopReason == "success",
		StopReason:  result.StopReason,
		ToolCalls:   result.ToolCalls,
	}

	learning.ExecutionHistory = append(learning.ExecutionHistory, record)
	learning.ExecutionCount++

	return record
}

func (a *SelfEvolvingAgent) analyzeExecution(ctx context.Context, execution ExecutionRecord, learning *SessionLearning) {
	// Basic implicit analysis
	signals := []LearningSignal{}

	// Check for iteration inefficiency
	if execution.Iterations > 10 {
		signals = append(signals, LearningSignal{
			Type:        SignalInefficiency,
			Severity:    0.6,
			Description: "High iteration count suggests inefficient planning",
		})
	}

	// Check for token inefficiency
	if execution.TokensUsed > 50000 {
		signals = append(signals, LearningSignal{
			Type:        SignalTokenExcess,
			Severity:    0.5,
			Description: "High token usage suggests verbose or redundant reasoning",
		})
	}

	// Check for error patterns in stop reason
	if execution.StopReason == "error" || execution.StopReason == "timeout" {
		signals = append(signals, LearningSignal{
			Type:        SignalPattern,
			Severity:    0.8,
			Description: "Execution ended with error - analyze failure pattern",
		})
	}

	// Store signals for later evolution
	for _, signal := range signals {
		record := LearningRecord{
			ID:         generateLearningID(),
			Timestamp:  time.Now(),
			SessionID:  learning.SessionID,
			Type:       LearningTypePerformance,
			Trigger:    LearningTriggerPattern,
			Signal:     signal,
			SourcePattern: fmt.Sprintf("execution_%s", execution.ID),
			Insight: Insight{
				Description: signal.Description,
				Confidence:  signal.Severity,
				Category:    signal.Type,
				Applicable:  true,
				CreatedAt:   time.Now(),
			},
		}

		if err := a.evolutionEngine.LearningStore().Store(ctx, record); err != nil {
			a.logger.Error("Failed to store implicit learning", "error", err)
		}
	}
}

func (a *SelfEvolvingAgent) shouldTriggerEvolution(learning *SessionLearning) bool {
	if !a.config.EnableAutoEvolution {
		return false
	}

	// Check cooldown
	if time.Since(learning.LastEvolutionTime) < a.config.EvolutionCooldown {
		return false
	}

	// Check minimum samples
	if learning.ExecutionCount < a.config.MinSamplesForEvolve {
		return false
	}

	// Check daily limit
	todayCount := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, record := range a.evolutionHistory {
		if record.Timestamp.After(cutoff) {
			todayCount++
		}
	}
	if todayCount >= a.config.MaxEvolutionsPerDay {
		return false
	}

	return true
}

func (a *SelfEvolvingAgent) triggerEvolution(ctx context.Context, sessionID string) error {
	a.logger.Info("Triggering evolution", "session", sessionID)

	result, err := a.evolutionEngine.RunEvolutionCycle(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("evolution cycle failed: %w", err)
	}

	// Update session learning state
	learning := a.getOrCreateSessionLearning(sessionID)
	learning.LastEvolutionTime = time.Now()
	learning.Generation = result.Generation

	if result.Success {
		learning.AppliedLearnings += len(result.Improvements)
		learning.PendingImprovements = result.Improvements
	}

	// Record evolution
	record := SessionEvolutionRecord{
		Timestamp:    time.Now(),
		SessionID:    sessionID,
		Generation:   result.Generation,
		Success:      result.Success,
		Improvements: result.Improvements,
		Insights:     result.Insights,
	}
	a.evolutionHistory = append(a.evolutionHistory, record)

	a.logger.Info("Evolution completed",
		"session", sessionID,
		"generation", result.Generation,
		"improvements", len(result.Improvements),
		"success", result.Success)

	return nil
}

// Helper structs for internal state

type SessionLearning struct {
	SessionID           string
	Generation          int
	ExecutionCount      int
	ExecutionHistory    []ExecutionRecord
	FeedbackHistory     []FeedbackEvent
	AppliedLearnings    int
	PendingImprovements []Improvement
	CreatedAt           time.Time
	LastEvolutionTime   time.Time
}

type ExecutionRecord struct {
	ID         string
	Timestamp  time.Time
	Task       string
	Result     string
	Iterations int
	TokensUsed int
	Duration   time.Duration
	Success    bool
	StopReason string
	ToolCalls  []ports.ToolCall
}

type ExecutionFeedback struct {
	RunID                string
	Rating               int       // 1-5
	Category             string
	Comment              string
	SuggestedImprovement string
	Source               string
}

// FeedbackEvent represents a feedback event for a session
type FeedbackEvent struct {
	ID        string
	Timestamp time.Time
	RunID     string
	Rating    int
	Comment   string
	Category  string
	Source    string
}

// Improvement represents a specific improvement made during evolution
type Improvement struct {
	ID          string
	Type        string
	Description string
	Category    string
	Confidence  float64
	Before      string
	After       string
}

// Insight represents a learning insight from evolution
type Insight struct {
	ID          string
	Type        string
	Description string
	Confidence  float64
	Source      string
}

type EvolutionStatus struct {
	SessionID           string
	TotalExecutions     int
	TotalFeedbacks      int
	LearningsApplied    int
	CurrentGeneration   int
	LastEvolutionTime   time.Time
	PendingImprovements []Improvement
	EvolutionHistory    []SessionEvolutionRecord
}

type SessionEvolutionRecord struct {
	Timestamp    time.Time
	SessionID    string
	Generation   int
	Success      bool
	Improvements []Improvement
	Insights     []Insight
}

// Helper functions
func generateExecutionID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}

func generateFeedbackID() string {
	return fmt.Sprintf("fb_%d", time.Now().UnixNano())
}

func generateLearningID() string {
	return fmt.Sprintf("learn_%d", time.Now().UnixNano())
}

// noopLogger is a no-op logger implementation.
type noopLogger struct{}

func (l *noopLogger) Debug(msg string, args ...any) {}
func (l *noopLogger) Info(msg string, args ...any)  {}
func (l *noopLogger) Warn(msg string, args ...any)  {}
func (l *noopLogger) Error(msg string, args ...any) {}

// Ensure JSON serialization works
func init() {
	// Test that EvolutionStatus can be serialized
	_ = EvolutionStatus{}
	_, _ = json.Marshal(EvolutionStatus{})
}

