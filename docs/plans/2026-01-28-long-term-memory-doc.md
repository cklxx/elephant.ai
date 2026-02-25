# Plan: Add long-term memory doc + daily refresh (2026-01-28)

## Context
- Need a dedicated memory document for long-lived knowledge.
- Must store update timestamps to hour precision.
- First load per day should re-rank and refresh memory using existing rules.

## Plan
1. Add a long-term memory document template with hour-level update timestamp.
2. Update AGENTS.md to include the new memory doc and daily refresh rule.
3. Extend memory automation spec to cover the long-term doc and daily refresh behavior.
4. Run full lint + tests and commit.

## Progress
- 2026-01-28: Plan created.
- 2026-01-28: Added long-term memory doc and AGENTS.md guidance; updated automation spec.
- 2026-01-28: Simplified AGENTS.md memory sources wording.
