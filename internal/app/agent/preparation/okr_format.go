package preparation

import (
	"fmt"
	"strings"

	"alex/internal/infra/tools/builtin/okr"
)

// FormatOKRGoalsSummary renders a Markdown summary of active OKR goals.
// Shared by OKRContextProvider (system prompt injection) and OKRContextHook
// (pre-task injection).
func FormatOKRGoalsSummary(goals []*okr.GoalFile) string {
	if len(goals) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("**Active OKR Goals:**\n\n")

	for _, goal := range goals {
		sb.WriteString(fmt.Sprintf("### %s", goal.Meta.ID))
		if goal.Meta.TimeWindow.Start != "" && goal.Meta.TimeWindow.End != "" {
			sb.WriteString(fmt.Sprintf(" (%s → %s)", goal.Meta.TimeWindow.Start, goal.Meta.TimeWindow.End))
		}
		sb.WriteString("\n")

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

		if goal.Meta.ReviewCadence != "" {
			sb.WriteString(fmt.Sprintf("- Review cadence: `%s`\n", goal.Meta.ReviewCadence))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
