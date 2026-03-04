# Reuse Precheck Guide

Updated: 2026-03-04

Use this guide before adding any new file or component.

## 1. Mandatory Workflow

1. Classify intended file by type and responsibility.
2. Resolve required directory from `docs/reference/reuse-catalog-and-folder-governance.md`.
3. Search existing capabilities using `docs/reference/reuse-path-index.md`.
4. Decide one action: `reuse`, `extend`, or `new`.
5. If action is `new`, record replacement reason with path evidence.

## 2. Required Commands

```bash
# Global reuse scan
rg -n "run_tasks|taskfile|status\\.yaml|TeamDefinition|tmux|process manager|runtime_env_loader|envmerge|wrapper" internal scripts docs configs

# File placement sanity check (advisory script)
bash scripts/check-reuse-precheck.sh
```

## 3. File Placement Rules by Type

| If you add... | Must be placed in... |
| --- | --- |
| runtime Go logic | `internal/**` |
| Go entrypoint | `cmd/**` |
| package unit tests | same package directory (`*_test.go`) |
| integration tests | `tests/**` or existing integration test location |
| runtime config YAML | `configs/**` |
| task input YAML | `tasks/**` |
| task status sidecar | `tasks/**` or `.elephant/tasks/**` with `.status.yaml` suffix |
| canonical governance/spec docs | `docs/reference/**` |
| operation/how-to docs | `docs/guides/**` |
| plan/progress docs | `docs/plans/**` |
| automation scripts | `scripts/**` |

## 3.1 Internal Routing Rules (Required for `internal/**`)

| If responsibility is... | Must place in... | Do not place in... |
| --- | --- | --- |
| domain model/rules/ports | `internal/domain/**` | `internal/infra/**`, `internal/delivery/**` |
| app-layer orchestration/use-case composition | `internal/app/**` | `internal/domain/**` for orchestration wiring |
| external adapters/providers/tool concrete impl | `internal/infra/**` | `internal/domain/**` |
| transport/channel/http/sse presentation | `internal/delivery/**` | `internal/infra/**`, `internal/shared/**` |
| generic shared primitives | `internal/shared/**` | feature-specific folders |
| local dev supervisor/process control | `internal/devops/**` | `internal/domain/**`, `internal/delivery/**` |
| test-only helper library | `internal/testutil/**` | runtime production paths |

Rule: adding `internal/<new-root>/...` is not allowed without architecture-review evidence in `docs/plans/**`.

## 4. Replacement Reason Template (Required for `new`)

```text
Existing capability checked: <path list>
Reuse decision: new
Why not reusable: <single concrete contract mismatch>
Convergence plan: <future merge path or none>
```

## 5. Review Checklist

1. No parallel orchestration tool was added.
2. No status format other than `.status.yaml` was introduced.
3. No new env parser bypassing shared config loader was added.
4. New files follow file-type placement rules.
5. Canonical docs were updated for behavior/contract changes.

## 6. Expected Outcomes

- Repeated implementation patterns are reduced.
- File placement is deterministic and reviewable.
- New contributors can locate the single source of truth quickly.
