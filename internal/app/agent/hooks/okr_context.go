package hooks

import (
	"context"

	"alex/internal/app/agent/preparation"
	"alex/internal/infra/tools/builtin/okr"
	"alex/internal/shared/logging"
)

// OKRContextConfig configures the OKR context injection hook.
type OKRContextConfig struct {
	Enabled    bool
	AutoInject bool
}

// OKRContextHook scans active goals and injects OKR context before task execution.
type OKRContextHook struct {
	store  *okr.GoalStore
	config OKRContextConfig
	logger logging.Logger
}

// NewOKRContextHook creates a new OKR context injection hook.
func NewOKRContextHook(store *okr.GoalStore, logger logging.Logger, config OKRContextConfig) *OKRContextHook {
	return &OKRContextHook{
		store:  store,
		config: config,
		logger: logging.OrNop(logger),
	}
}

func (h *OKRContextHook) Name() string {
	return "okr_context"
}

func (h *OKRContextHook) OnTaskStart(_ context.Context, _ TaskInfo) []Injection {
	if !h.config.Enabled || !h.config.AutoInject {
		return nil
	}

	goals, err := h.store.ListActiveGoals()
	if err != nil {
		h.logger.Warn("OKR context hook: failed to list active goals: %v", err)
		return nil
	}

	if len(goals) == 0 {
		return nil
	}

	content := preparation.FormatOKRGoalsSummary(goals)
	if content == "" {
		return nil
	}

	h.logger.Debug("OKR context hook: injecting %d active goals", len(goals))

	return []Injection{
		{
			Type:     injectionOKRContext,
			Content:  content,
			Source:   "okr_context",
			Priority: 80,
		},
	}
}

func (h *OKRContextHook) OnTaskCompleted(_ context.Context, _ TaskResultInfo) error {
	return nil
}
