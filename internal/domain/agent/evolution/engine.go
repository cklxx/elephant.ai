//go:build ignore
// Copyright (c) 2025 elephant.ai
// SPDX-License-Identifier: MIT

package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/analytics/journal"
)

// Engine orchestrates the self-evolution lifecycle of an agent.
type Engine struct {
	config     Config
	analyzer   *PerformanceAnalyzer
	optimizer  *PromptOptimizer
	storage    LearningStorage
	journal    *journal.Writer
	logger     *slog.Logger
	stopCh     chan struct{}
	evolutionCh chan EvolutionEvent
}

// EngineOption configures the Engine.
type EngineOption func(*Engine)

// WithEngineLogger sets the logger for the engine.
func WithEngineLogger(logger *slog.Logger) EngineOption {
	return func(e *Engine) {
		e.logger = logger
	}
}

// WithEngineJournal sets the journal for recording evolution events.
func WithEngineJournal(j *journal.Writer) EngineOption {
	return func(e *Engine) {
		e.journal = j
	}
}

// NewEngine creates a new evolution engine.
func NewEngine(cfg Config, storage LearningStorage, opts ...EngineOption) *Engine {
	if storage == nil {
		storage = NewInMemoryLearningStorage()
	}

	eng := &Engine{
		config:      cfg,
		analyzer:    NewPerformanceAnalyzer(cfg),
		optimizer:   NewPromptOptimizer(cfg),
		storage:     storage,
		logger:      slog.Default(),
		stopCh:      make(chan struct{}),
		evolutionCh: make(chan EvolutionEvent, 100),
	}

	for _, opt := range opts {
		opt(eng)
	}

	return eng
}

// Start begins the background evolution worker.
func (e *Engine) Start(ctx context.Context) {
	go e.evolutionWorker(ctx)
	if e.config.AutoOptimizeInterval > 0 {
		go e.autoOptimizeWorker(ctx)
	}
}

// Stop signals the engine to stop.
func (e *Engine) Stop() {
	close(e.stopCh)
}

// evolutionWorker processes evolution events.
func (e *Engine) evolutionWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case event := <-e.evolutionCh:
			if err := e.processEvolutionEvent(ctx, event); err != nil {
				e.logger.Warn("Failed to process evolution event", "error", err)
			}
		}
	}
}

// autoOptimizeWorker periodically triggers automatic optimization.
func (e *Engine) autoOptimizeWorker(ctx context.Context) {
	ticker := time.NewTicker(e.config.AutoOptimizeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			if err := e.TriggerAutoOptimization(ctx); err != nil {
				e.logger.Warn("Auto-optimization failed", "error", err)
			}
		}
	}
}

// RecordExecution records a task execution for analysis.
func (e *Engine) RecordExecution(ctx context.Context, execution TaskExecution) error {
	// Store execution
	if err := e.storage.SaveExecution(ctx, execution); err != nil {
		return fmt.Errorf("save execution: %w", err)
	}

	// Perform real-time analysis
	metrics := e.analyzer.AnalyzeSingle(execution)
	
	// Store feedback
	feedback := FeedbackItem{
		ID:          generateFeedbackID(),
		ExecutionID: execution.ID,
		Type:        FeedbackTypeAuto,
		Score:       metrics.QualityScore,
		Metrics:     metrics,
		CreatedAt:   time.Now(),
	}
	
	if err := e.storage.SaveFeedback(ctx, feedback); err != nil {
		e.logger.Warn("Failed to save auto feedback", "error", err)
	}

	// Emit evolution event
	e.evolutionCh <- EvolutionEvent{
		Type:      EventTypeExecutionRecorded,
		Payload:   execution,
		Timestamp: time.Now(),
	}

	return nil
}

// RecordUserFeedback records explicit user feedback.
func (e *Engine) RecordUserFeedback(ctx context.Context, executionID string, score float64, comment string, dimensions map[string]float64) error {
	feedback := FeedbackItem{
		ID:          generateFeedbackID(),
		ExecutionID: executionID,
		Type:        FeedbackTypeExplicit,
		Score:       score,
		Comment:     comment,
		Dimensions:  dimensions,
		CreatedAt:   time.Now(),
	}

	if err := e.storage.SaveFeedback(ctx, feedback); err != nil {
		return fmt.Errorf("save feedback: %w", err)
	}

	e.evolutionCh <- EvolutionEvent{
		Type:      EventTypeFeedbackReceived,
		Payload:   feedback,
		Timestamp: time.Now(),
	}

	return nil
}

// RecordPatternMatch records a successfully matched pattern.
func (e *Engine) RecordPatternMatch(ctx context.Context, patternID string, executionID string, matchData map[string]any) error {
	pattern, err := e.storage.GetPattern(ctx, patternID)
	if err != nil {
		return fmt.Errorf("get pattern: %w", err)
	}

	pattern.MatchCount++
	pattern.LastMatchedAt = time.Now()
	
	if err := e.storage.SavePattern(ctx, pattern); err != nil {
		return fmt.Errorf("update pattern: %w", err)
	}

	e.logger.Info("Pattern matched",
		"pattern_id", patternID,
		"execution_id", executionID,
		"total_matches", pattern.MatchCount,
	)

	return nil
}

// processEvolutionEvent handles individual evolution events.
func (e *Engine) processEvolutionEvent(ctx context.Context, event EvolutionEvent) error {
	switch event.Type {
	case EventTypeExecutionRecorded:
		return e.handleExecutionRecorded(ctx, event.Payload.(TaskExecution))
	case EventTypeFeedbackReceived:
		return e.handleFeedbackReceived(ctx, event.Payload.(FeedbackItem))
	case EventTypeOptimizationTriggered:
		return e.handleOptimizationTriggered(ctx)
	default:
		return nil
	}
}

// handleExecutionRecorded processes a newly recorded execution.
func (e *Engine) handleExecutionRecorded(ctx context.Context, execution TaskExecution) error {
	// Check if we need to trigger optimization
	count, err := e.storage.GetExecutionCount(ctx)
	if err != nil {
		return err
	}

	if count >= e.config.MinExecutionsBeforeOptimize && count%e.config.OptimizationInterval == 0 {
		e.evolutionCh <- EvolutionEvent{
			Type:      EventTypeOptimizationTriggered,
			Payload:   nil,
			Timestamp: time.Now(),
		}
	}

	return nil
}

// handleFeedbackReceived processes user feedback.
func (e *Engine) handleFeedbackReceived(ctx context.Context, feedback FeedbackItem) error {
	// Extract learnings from feedback
	if feedback.Score < 0.5 {
		// Poor performance - record as negative pattern
		execution, err := e.storage.GetExecution(ctx, feedback.ExecutionID)
		if err != nil {
			return err
		}

		pattern := LearnedPattern{
			ID:          generatePatternID(),
			Type:        PatternTypeAnti,
			Description: fmt.Sprintf("Low quality execution (score: %.2f): %s", feedback.Score, feedback.Comment),
			Conditions: map[string]any{
				"task_type": execution.TaskType,
				"tools_used": execution.ToolsUsed,
			},
			Response:    map[string]any{"warning": "This pattern led to low quality results"},
			Confidence:  0.7,
			CreatedAt:   time.Now(),
			SourceRunID: execution.ID,
		}

		if err := e.storage.SavePattern(ctx, pattern); err != nil {
			return err
		}
	}

	return nil
}

// handleOptimizationTriggered performs prompt optimization.
func (e *Engine) handleOptimizationTriggered(ctx context.Context) error {
	return e.TriggerAutoOptimization(ctx)
}

// TriggerAutoOptimization runs automatic optimization based on collected data.
func (e *Engine) TriggerAutoOptimization(ctx context.Context) error {
	e.logger.Info("Starting auto-optimization")

	// Get recent executions
	executions, err := e.storage.ListExecutions(ctx, 100, 0)
	if err != nil {
		return fmt.Errorf("list executions: %w", err)
	}

	if len(executions) < e.config.MinExecutionsBeforeOptimize {
		e.logger.Info("Not enough executions for optimization", "count", len(executions))
		return nil
	}

	// Analyze performance trends
	trend := e.analyzer.AnalyzeTrend(executions)
	
	// Get current prompt
	prompts, err := e.storage.ListPromptVersions(ctx, "", 1, 0)
	if err != nil || len(prompts) == 0 {
		e.logger.Warn("No current prompt found for optimization")
		return nil
	}
	currentPrompt := prompts[0]

	// Check if optimization is needed
	if trend.AverageScore >= e.config.QualityThreshold {
		e.logger.Info("Quality threshold met, skipping optimization", "score", trend.AverageScore)
		return nil
	}

	// Collect learnings
	learnings, err := e.collectLearnings(ctx)
	if err != nil {
		e.logger.Warn("Failed to collect learnings", "error", err)
	}

	// Generate optimization
	req := OptimizationRequest{
		CurrentPrompt: currentPrompt.Content,
		Learnings:     learnings,
		Trend:         trend,
		Strategy:      OptimizationStrategyIterative,
	}

	result, err := e.optimizer.Optimize(ctx, req)
	if err != nil {
		return fmt.Errorf("optimization failed: %w", err)
	}

	if !result.ShouldApply {
		e.logger.Info("Optimization deemed unnecessary", "reason", result.Reason)
		return nil
	}

	// Save new prompt version
	newVersion := PromptVersion{
		ID:             generatePromptID(),
		AgentType:      currentPrompt.AgentType,
		Content:        result.NewPrompt,
		ParentVersion:  currentPrompt.ID,
		ChangeType:     changeTypeFromStrategy(result.Strategy),
		ChangeSummary:  result.ChangeSummary,
		Performance:    result.EstimatedImpact,
		CreatedAt:      time.Now(),
		OptimizationID: result.OptimizationID,
	}

	if err := e.storage.SavePromptVersion(ctx, newVersion); err != nil {
		return fmt.Errorf("save prompt version: %w", err)
	}

	// Record in journal
	if e.journal != nil {
		e.journal.Write(ctx, journal.TurnJournalEntry{
			SessionID: "evolution_" + newVersion.ID,
			Summary:   fmt.Sprintf("Prompt optimized: %s", result.ChangeSummary),
			Plans: []map[string]any{
				{"optimization_id": result.OptimizationID, "estimated_impact": result.EstimatedImpact},
			},
		})
	}

	e.logger.Info("Optimization completed",
		"version_id", newVersion.ID,
		"strategy", result.Strategy,
		"estimated_impact", result.EstimatedImpact,
	)

	return nil
}

// collectLearnings gathers all learning data for optimization.
func (e *Engine) collectLearnings(ctx context.Context) ([]LearningEntry, error) {
	var learnings []LearningEntry

	// Collect patterns
	patterns, err := e.storage.ListPatterns(ctx, 50, 0)
	if err != nil {
		return nil, err
	}

	for _, pattern := range patterns {
		learnings = append(learnings, LearningEntry{
			Type:       LearningTypePattern,
			SourceID:   pattern.ID,
			Content:    pattern.Description,
			Confidence: pattern.Confidence,
			Timestamp:  pattern.CreatedAt,
		})
	}

	// Collect mistakes
	mistakes, err := e.storage.ListMistakes(ctx, 50, 0)
	if err != nil {
		return nil, err
	}

	for _, mistake := range mistakes {
		learnings = append(learnings, LearningEntry{
			Type:       LearningTypeMistake,
			SourceID:   mistake.ID,
			Content:    mistake.Description,
			Confidence: float64(mistake.Severity) / 5.0,
			Timestamp:  mistake.CreatedAt,
		})
	}

	return learnings, nil
}

// GetCurrentPrompt returns the current optimized prompt for an agent.
func (e *Engine) GetCurrentPrompt(ctx context.Context, agentType string) (string, error) {
	prompts, err := e.storage.ListPromptVersions(ctx, agentType, 1, 0)
	if err != nil {
		return "", err
	}

	if len(prompts) == 0 {
		return "", fmt.Errorf("no prompt found for agent type: %s", agentType)
	}

	return prompts[0].Content, nil
}

// RollbackPrompt rolls back to a previous prompt version.
func (e *Engine) RollbackPrompt(ctx context.Context, versionID string) error {
	version, err := e.storage.GetPromptVersion(ctx, versionID)
	if err != nil {
		return fmt.Errorf("get prompt version: %w", err)
	}

	// Mark as rolled back
	version.RolledBack = true
	version.RolledBackAt = time.Now()

	if err := e.storage.SavePromptVersion(ctx, version); err != nil {
		return fmt.Errorf("save rolled back version: %w", err)
	}

	// Get parent version to restore
	if version.ParentVersion == "" {
		return fmt.Errorf("no parent version to rollback to")
	}

	parent, err := e.storage.GetPromptVersion(ctx, version.ParentVersion)
	if err != nil {
		return fmt.Errorf("get parent version: %w", err)
	}

	// Create new version based on parent
	restored := PromptVersion{
		ID:            generatePromptID(),
		AgentType:     parent.AgentType,
		Content:       parent.Content,
		ParentVersion: version.ID,
		ChangeType:    PromptChangeRollback,
		ChangeSummary: fmt.Sprintf("Rolled back from %s", versionID),
		CreatedAt:     time.Now(),
	}

	if err := e.storage.SavePromptVersion(ctx, restored); err != nil {
		return fmt.Errorf("save restored version: %w", err)
	}

	e.logger.Info("Prompt rolled back", "from", versionID, "to", restored.ID)
	return nil
}

// GetEvolutionReport generates a comprehensive evolution report.
func (e *Engine) GetEvolutionReport(ctx context.Context) (EvolutionReport, error) {
	executions, err := e.storage.ListExecutions(ctx, 1000, 0)
	if err != nil {
		return EvolutionReport{}, err
	}

	patterns, err := e.storage.ListPatterns(ctx, 100, 0)
	if err != nil {
		return EvolutionReport{}, err
	}

	prompts, err := e.storage.ListPromptVersions(ctx, "", 100, 0)
	if err != nil {
		return EvolutionReport{}, err
	}

	trend := e.analyzer.AnalyzeTrend(executions)

	return EvolutionReport{
		GeneratedAt:       time.Now(),
		TotalExecutions:   len(executions),
		TotalPatterns:     len(patterns),
		TotalPromptVersions: len(prompts),
		Trend:             trend,
		LearnedPatterns:   patterns,
		PromptVersions:    prompts,
	}, nil
}

// Helper functions

func generatePromptID() string {
	return fmt.Sprintf("prm_%d", time.Now().UnixNano())
}

func changeTypeFromStrategy(strategy OptimizationStrategy) PromptChangeType {
	switch strategy {
	case OptimizationStrategyConservative:
		return PromptChangeMinor
	case OptimizationStrategyIterative:
		return PromptChangeRefinement
	case OptimizationStrategyAggressive:
		return PromptChangeMajor
	default:
		return PromptChangeRefinement
	}
}

