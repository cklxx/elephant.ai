package decision

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/shared/utils/id"
)

// LearnablePattern is the input type for LearnPatterns.
// Decouples the decision engine from the infra/memory/distillation package.
type LearnablePattern struct {
	Category    string
	Description string
	Confidence  float64
	Evidence    []string
}

// Engine observes decisions, learns patterns, and can auto-act.
type Engine struct {
	decisions      *Store
	patterns       *PatternStore
	autoActEnabled bool
	auditLog       *AuditLog
	nowFn          func() time.Time
	mu             sync.RWMutex
}

// NewEngine creates a decision Engine.
func NewEngine(decisions *Store, patterns *PatternStore, auditLog *AuditLog, nowFn func() time.Time) *Engine {
	return &Engine{
		decisions: decisions, patterns: patterns,
		auditLog: auditLog, nowFn: nowFn,
	}
}

// SetAutoAct enables or disables the auto-act feature flag.
func (e *Engine) SetAutoAct(enabled bool) {
	e.mu.Lock()
	e.autoActEnabled = enabled
	e.mu.Unlock()
}

// Observe records a new decision and updates any matching pattern.
func (e *Engine) Observe(ctx context.Context, d *Decision) error {
	if err := e.decisions.Add(d); err != nil {
		return fmt.Errorf("store decision: %w", err)
	}
	e.updateMatchingPattern(d)
	return nil
}

// ShouldAutoAct checks if there's a high-confidence pattern for this decision type.
func (e *Engine) ShouldAutoAct(_ context.Context, category, condition string) (*Pattern, bool) {
	e.mu.RLock()
	enabled := e.autoActEnabled
	e.mu.RUnlock()

	if !enabled {
		return nil, false
	}
	return e.findAutoActPattern(category, condition)
}

func (e *Engine) findAutoActPattern(category, condition string) (*Pattern, bool) {
	matches := e.patterns.FindMatching(category, condition)
	for _, p := range matches {
		if p.Confidence >= ConfidenceFloor {
			return p, true
		}
	}
	return nil, false
}

// RecordCorrection reduces confidence when a user overrides an auto-action.
func (e *Engine) RecordCorrection(_ context.Context, patternID string, correction string) error {
	p := e.patterns.Get(patternID)
	if p == nil {
		return fmt.Errorf("pattern %q not found", patternID)
	}
	p.Confidence *= 0.8
	p.SampleCount++
	p.UpdatedAt = e.nowFn()
	if err := e.patterns.Save(p); err != nil {
		return fmt.Errorf("save corrected pattern: %w", err)
	}
	return e.auditLog.Record(AuditEntry{
		ID: id.NewKSUID(), PatternID: patternID, Category: p.Category,
		Action: correction, Confidence: p.Confidence, AutoActed: true,
		Corrected: true, Correction: correction,
		UndoBy: e.nowFn().Add(undoWindow), CreatedAt: e.nowFn(),
	})
}

// LearnPatterns creates or updates patterns from distillation output.
func (e *Engine) LearnPatterns(_ context.Context, weeklyPatterns []LearnablePattern) error {
	now := e.nowFn()
	for _, wp := range weeklyPatterns {
		p := &Pattern{
			ID: id.NewKSUID(), Category: wp.Category,
			Description: wp.Description, Condition: wp.Description,
			Action: "auto", Confidence: wp.Confidence,
			SampleCount: len(wp.Evidence),
			CreatedAt: now, UpdatedAt: now,
		}
		if err := e.patterns.Save(p); err != nil {
			return fmt.Errorf("save learned pattern: %w", err)
		}
	}
	return nil
}

func (e *Engine) updateMatchingPattern(d *Decision) {
	category := categoryFromTags(d.Tags)
	matches := e.patterns.FindMatching(category, d.Description)
	if len(matches) == 0 {
		return
	}
	p := matches[0]
	p.SampleCount++
	p.LastMatched = e.nowFn()
	p.UpdatedAt = e.nowFn()
	_ = e.patterns.Save(p)
}

func categoryFromTags(tags []string) string {
	if len(tags) > 0 {
		return tags[0]
	}
	return "general"
}
