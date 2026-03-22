package preparation

import (
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

		return FormatOKRGoalsSummary(goals)
	}
}
