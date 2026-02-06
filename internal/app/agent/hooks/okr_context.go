package hooks

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/logging"
	"alex/internal/tools/builtin/okr"
)

// OKRContextConfig configures the OKR context injection hook.
type OKRContextConfig struct {
	Enabled    bool
	AutoInject bool
}

// OKRContextHook scans active goals and injects OKR context before task execution.
type OKRContextHook struct {
	store   *okr.GoalStore
	config  OKRContextConfig
	logger  logging.Logger
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

	content := h.buildOKRSummary(goals)
	if content == "" {
		return nil
	}

	h.logger.Debug("OKR context hook: injecting %d active goals", len(goals))

	return []Injection{
		{
			Type:     InjectionOKRContext,
			Content:  content,
			Source:   "okr_context",
			Priority: 80,
		},
	}
}

func (h *OKRContextHook) OnTaskCompleted(_ context.Context, _ TaskResultInfo) error {
	return nil
}

func (h *OKRContextHook) buildOKRSummary(goals []*okr.GoalFile) string {
	var sb strings.Builder
	sb.WriteString("**Active OKR Goals:**\n\n")

	for _, goal := range goals {
		sb.WriteString(fmt.Sprintf("### %s", goal.Meta.ID))
		if goal.Meta.TimeWindow.Start != "" && goal.Meta.TimeWindow.End != "" {
			sb.WriteString(fmt.Sprintf(" (%s → %s)", goal.Meta.TimeWindow.Start, goal.Meta.TimeWindow.End))
		}
		sb.WriteString("\n")

		if len(goal.Meta.KeyResults) > 0 {
			for krID, kr := range goal.Meta.KeyResults {
				freshness := ""
				if kr.Updated != "" {
					freshness = fmt.Sprintf(" (data from %s)", kr.Updated)
				}
				riskIcon := "✓"
				if kr.ProgressPct < 25 {
					riskIcon = "⚠"
				} else if kr.ProgressPct < 10 {
					riskIcon = "✗"
				}
				sb.WriteString(fmt.Sprintf("- %s **%s**: %.0f/%.0f (%.1f%%) %s%s\n",
					riskIcon, krID, kr.Current, kr.Target, kr.ProgressPct, kr.Confidence, freshness))
			}
		}

		if goal.Meta.ReviewCadence != "" {
			sb.WriteString(fmt.Sprintf("- Review cadence: `%s`\n", goal.Meta.ReviewCadence))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
