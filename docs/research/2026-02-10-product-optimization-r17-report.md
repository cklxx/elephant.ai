# Product Optimization R17 Report (2026-02-10)

## Scope
- Objective: improve real product routing capability (implicit intent + tool discoverability/usability), not eval-only boost tuning.
- Constraints: preserve `pass@5`, keep deliverable quality stable, and keep sandbox/local tool semantics aligned.

## Runs
- Baseline run: `tmp/foundation-suite-r17-baseline-20260210-010149`
- Iteration-1 run: `tmp/foundation-suite-r17-opt1-20260210-011153`
- Final run: `tmp/foundation-suite-r17-final-20260210-011820`

## Scoreboard (x/x)
- Baseline:
  - pass@1: `216/257`
  - pass@5: `257/257`
  - deliverable good: `22/25`
- Final:
  - pass@1: `225/257`
  - pass@5: `257/257`
  - deliverable good: `22/25`
- Delta:
  - pass@1: `+9`
  - pass@5: `0`
  - deliverable good: `0`

## Product Changes
- Tool semantic boundary convergence (local + sandbox parity):
  - file/content routing: `read_file`, `search_file`, `find`, `list_dir`, `replace_in_file`, `write_attachment`
  - memory/lark/okr routing: `memory_get`, `lark_chat_history`, `okr_read`
  - artifact lifecycle routing: `artifacts_write`, `artifacts_list`, `artifact_manifest`
  - browser/web routing: `browser_dom`, `browser_screenshot`, `web_search`, `web_fetch`
  - scheduler/timer disambiguation: `scheduler_delete_job`, `cancel_timer`
- Prompt routing guardrails strengthened:
  - `internal/app/context/manager_prompt.go`
  - `internal/shared/agent/presets/prompts.go`
  - `internal/app/agent/preparation/service.go`
- Regression tests expanded:
  - added `internal/infra/tools/builtin/web/routing_descriptions_test.go`
  - updated routing description tests across aliases/sandbox/browser/larktools/memory

## Top Remaining Top1 Conflict Clusters
- `read_file => memory_get`: `5`
- `search_file => find`: `3`
- `search_file => browser_screenshot`: `2`
- `artifacts_list => artifacts_write`: `2`
- long-tail singletons (each `1`): include `web_fetch => web_search`, `scheduler_delete_job => plan/replace_in_file`, `execute_code => okr_read`, etc.

## Failure Analysis (Representative)
- `read_file => memory_get`
  - Pattern: intents like "open exact ... context window" in hard research/coding tasks.
  - Why: lexical overlap still biases memory line-retrieval semantics.
  - Next fix direction: shift from lexical-only weighting toward stronger repository-context priors (path-known + code context verbs).
- `search_file => find`
  - Pattern: "semantic body evidence, not filename" still occasionally pulled by "find" verb.
  - Why: action verb token can dominate body-evidence intent.
  - Next fix direction: increase penalty when intent explicitly negates path/name evidence.
- `artifacts_list => artifacts_write`
  - Pattern: inventory-before-release intents near deliverable vocabulary.
  - Why: shared artifact nouns still trigger write path in top1.
  - Next fix direction: add stronger inventory/selection gating before any mutation tools.

## Validation
- Formatting/lint: `make fmt` passed.
- Full tests: `make test` passed.
- Suite validation: foundation suite final run completed, `failed_cases=0`.

## Conclusion
- This round achieved measurable product capability lift (`pass@1 +9`) without sacrificing `pass@5` or delivery quality.
- Remaining misses are concentrated in a few stable lexical-conflict families; further gains require deeper router scoring policy upgrades, not only tool-description tuning.
