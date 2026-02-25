package preparation

import (
	"fmt"
	"strings"

	"alex/internal/infra/tools/builtin/okr"
)

// OKRContextProvider generates pre-rendered OKR context for system prompt injection.
type OKRContextProvider func() string

// NewOKRContextProvider creates an OKRContextProvider backed by a GoalStore.
// When active goals exist, it returns a formatted summary.
// When no goals exist, it returns a discovery prompt encouraging the agent to
// proactively suggest OKR creation.
func NewOKRContextProvider(store *okr.GoalStore) OKRContextProvider {
	return func() string {
		goals, err := store.ListActiveGoals()
		if err != nil {
			return ""
		}

		if len(goals) == 0 {
			return `No active OKR goals found.
When the user discusses goals, plans, quarterly priorities, or key results, proactively suggest creating OKR goals using okr_write.
Use okr_read to check whether any existing goals are available.`
		}

		return buildOKRGoalsSummary(goals)
	}
}

func buildOKRGoalsSummary(goals []*okr.GoalFile) string {
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
				if kr.ProgressPct < 10 {
					riskIcon = "✗"
				} else if kr.ProgressPct < 25 {
					riskIcon = "⚠"
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
