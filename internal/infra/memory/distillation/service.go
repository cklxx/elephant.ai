package distillation

import (
	"context"
	"fmt"
	"time"

	"alex/internal/infra/memory"
)

// Service orchestrates the distillation pipeline.
type Service struct {
	memoryEngine memory.Engine
	extractor    *Extractor
	analyzer     *PatternAnalyzer
	store        *Store
	nowFn        func() time.Time
}

// NewService creates a distillation Service.
func NewService(engine memory.Engine, extractor *Extractor, analyzer *PatternAnalyzer, store *Store, nowFn func() time.Time) *Service {
	return &Service{
		memoryEngine: engine, extractor: extractor,
		analyzer: analyzer, store: store, nowFn: nowFn,
	}
}

// RunDaily loads today's conversations, extracts facts, and stores them.
func (s *Service) RunDaily(ctx context.Context, userID string) error {
	today := s.nowFn()
	content, err := s.memoryEngine.LoadDaily(ctx, userID, today)
	if err != nil {
		return fmt.Errorf("load daily memory: %w", err)
	}
	if content == "" {
		return nil
	}

	date := today.Format("2006-01-02")
	extraction, err := s.extractor.ExtractDaily(ctx, content, date)
	if err != nil {
		return fmt.Errorf("extract daily: %w", err)
	}
	return s.store.SaveDailyExtraction(ctx, extraction)
}

// RunWeekly loads the past week's extractions, finds patterns, and stores them.
func (s *Service) RunWeekly(ctx context.Context, userID string) error {
	now := s.nowFn()
	weekStart := now.AddDate(0, 0, -7)
	extractions, err := s.store.ListDailyExtractions(ctx, weekStart, now)
	if err != nil {
		return fmt.Errorf("list extractions: %w", err)
	}
	if len(extractions) == 0 {
		return nil
	}

	patterns, err := s.analyzer.AnalyzeWeek(ctx, extractions)
	if err != nil {
		return fmt.Errorf("analyze week: %w", err)
	}

	weekKey := weekStart.Format("2006-01-02")
	return s.store.SaveWeeklyPatterns(ctx, weekKey, patterns)
}
