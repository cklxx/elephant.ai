# 2026-01-27 - dev test orchestration explore failure

- Summary: `./dev.sh test` failed in `internal/tools/builtin/orchestration` (`TestExploreDelegationFlow`, `TestExploreDefaultSubtaskWhenNoScopes`) with missing prompt strings.
- Likely cause: unrelated local changes in orchestration prompt wiring.
- Status: unresolved in this run.
