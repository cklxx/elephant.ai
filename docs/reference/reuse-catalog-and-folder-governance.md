# Reuse Catalog and Folder Governance

Updated: 2026-03-04

## 1. Scope

This document is the canonical policy for file-level placement, reuse boundaries, and anti-duplication review gates.

Hard priority for all changes:

1. safety
2. correctness
3. maintainability
4. speed

## 2. Non-Negotiable Rules

1. New capability must reuse existing implementation first. Do not add parallel implementations.
2. Every new file must match the file-type-to-directory mapping in this document.
3. Runtime state files must use existing sidecar conventions (`*.status.yaml`).
4. Configuration examples are YAML-only.
5. If reuse is impossible, record why with explicit path-level evidence.

## 3. File-Type to Directory Mapping (Mandatory)

| File type | Responsibility | Required location | Forbidden locations |
| --- | --- | --- | --- |
| `*.go` (runtime logic) | domain/app/infra/delivery/shared implementation | `internal/**` | `docs/**`, `tasks/**`, `artifacts/**`, repo root |
| `*.go` (entrypoint) | executable startup/command wiring | `cmd/**` | `internal/**` for CLI/server entrypoint files |
| `*_test.go` (unit) | package-level tests | same directory as tested package | unrelated test folders |
| `*_test.go` (cross-module integration) | cross-boundary behavior tests | `tests/**` or existing integration test directories | `docs/**`, `artifacts/**` |
| `*.yaml` (runtime config) | runtime/environment config | `configs/**` | `internal/**`, `scripts/**` as config source of truth |
| `*.yaml` (task input) | `run_tasks` input task files | `tasks/**` | `configs/**`, `docs/**` |
| `*.status.yaml` | task status sidecar | `tasks/**` (file mode) or `.elephant/tasks/**` (template/kernel mode) | any other folder |
| `*.md` (canonical spec) | architecture/contracts/governance truth | `docs/reference/**` | `docs/plans/**`, `docs/reviews/**` as canonical truth |
| `*.md` (how-to guide) | execution/operation procedure | `docs/guides/**` | `docs/reference/**` for procedure-only docs |
| `*.md` (plan/progress) | implementation plan and progress log | `docs/plans/**` | `docs/reference/**` |
| `*.md` (records) | historical reviews/research/analysis | `docs/reviews/**`, `docs/research/**`, `docs/analysis/**` | `docs/reference/**` |
| `*.sh` | shell automation scripts | `scripts/**` | repo root |
| `*.py` (bridge/tooling) | helper bridge or local tooling scripts | `scripts/**` | `internal/**` as runtime business code |
| runtime artifacts | logs/pids/tmp outputs | `logs/**`, `pids/**`, `tmp/**`, `.tmp/**`, `artifacts/**` | `internal/**`, `docs/**`, `configs/**` |

## 4. Internal Package Placement Matrix (No Ambiguity)

All new files under `internal/` must map to one and only one first-level namespace:

- `internal/app/`
- `internal/domain/`
- `internal/infra/`
- `internal/delivery/`
- `internal/shared/`
- `internal/devops/`
- `internal/testutil/`

Adding a new first-level namespace under `internal/` is forbidden without an architecture review record in `docs/plans/`.

### 4.1 First-Level Responsibility

| Path prefix | Allowed responsibility | Explicitly forbidden |
| --- | --- | --- |
| `internal/domain/` | pure business model, domain rules, domain ports/interfaces, domain events | provider SDK calls, file/network/DB side effects, HTTP handlers, CLI wiring |
| `internal/app/` | application orchestration/use-cases, DI-facing composition logic, cross-domain coordination | direct transport delivery code, provider-specific adapter logic |
| `internal/infra/` | concrete adapters to external systems (LLM, storage, process, tools, bridges, runtime) | domain policy decisions, UI/delivery protocol contracts |
| `internal/delivery/` | channel/transport entrypoints (server/http/sse/lark/telegram/output formatting) | domain rule implementation, provider-specific low-level clients |
| `internal/shared/` | cross-cutting generic primitives used by multiple layers (config, logging, json, parser, utils) | feature-specific business logic or channel-specific behavior |
| `internal/devops/` | local/dev runtime process orchestration and service lifecycle tooling | production domain logic, user-facing delivery handlers |
| `internal/testutil/` | reusable test-only helpers | runtime production paths |

### 4.2 Required Subdirectory Mapping

| If the new file does... | Must go under... | Example anchor paths |
| --- | --- | --- |
| ReAct loop coordination, agent execution orchestration | `internal/app/agent/**` | `internal/app/agent/coordinator/`, `internal/app/agent/kernel/` |
| Context assembly/compression and context-level app policy | `internal/app/context/**` | `internal/app/context/` |
| Dependency wiring/container construction | `internal/app/di/**` | `internal/app/di/` |
| Scheduler/reminder/notification app orchestration | `internal/app/scheduler/**`, `internal/app/reminder/**`, `internal/app/notification/**` | corresponding existing folders |
| Core task/workflow domain states and transitions | `internal/domain/task/**`, `internal/domain/workflow/**` | existing domain packages |
| Domain-level agent contracts/types | `internal/domain/agent/**` | `ports/`, `react/`, `taskfile/` |
| Tool implementations and concrete tool adapters | `internal/infra/tools/**` | `internal/infra/tools/builtin/` |
| External agent bridge/executor implementation | `internal/infra/external/**` | `internal/infra/external/bridge/` |
| Process/tmux backend internals | `internal/infra/process/**` | `tmux_backend.go`, `exec_backend.go` |
| HTTP/SSE router/handler/channel gateway | `internal/delivery/server/**`, `internal/delivery/channels/**` | existing delivery paths |
| Global config model/loading/normalization | `internal/shared/config/**` | `types.go`, `load.go`, `runtime_env_loader.go` |
| General utility package with no feature ownership | `internal/shared/**` | `logging/`, `json/`, `utils/` |
| Local service lifecycle supervisor/start-stop | `internal/devops/**` | `process/`, `services/`, `supervisor/` |
| Reusable test fixtures/helpers | `internal/testutil/**` | test helper packages |

### 4.3 Internal Anti-Duplication Rules

1. Do not create duplicate abstractions across `app` and `domain` for the same contract.
2. Do not place domain contracts in `infra`; adapters must depend on existing domain ports.
3. Do not add delivery-channel logic under `infra` or `shared`.
4. Do not add feature-specific helpers to `shared`; keep `shared` generic and reusable.
5. Do not create `internal/<new-root>/...` without architecture approval.

## 5. Directory Ownership Boundary

| Directory | Owner | Writable by runtime | Notes |
| --- | --- | --- | --- |
| `internal/` | backend engineers | no | source code only |
| `cmd/` | backend engineers | no | entrypoints only |
| `configs/` | platform/config owners | no | YAML only |
| `docs/reference/` | architecture owners | no | single source of truth |
| `docs/guides/` | developer productivity owners | no | executable workflows |
| `docs/plans/` | task implementer | no | must include progress/status |
| `scripts/` | devops/tooling owners | no | operational automation |
| `tasks/` | operators/agents | yes (sidecar only) | input YAML + file-mode sidecar |
| `.elephant/tasks/` | runtime | yes | template/kernel sidecar only |
| `artifacts/` | runtime and operators | yes | durable outputs only |
| `tmp/`, `.tmp/` | runtime and operators | yes | disposable temporary outputs |

Minimum writable set for runtime processes:

- `.elephant/tasks/`
- `tasks/` (only `*.status.yaml` sidecars)
- `logs/`
- `pids/`
- `tmp/` and `.tmp/`
- `artifacts/`

## 6. Naming Rules (Default)

### 5.1 Documents

- Canonical references and guides: `kebab-case.md`
- Plans and records: `YYYY-MM-DD-short-slug.md`

### 5.2 Task and Status Files

- Task input file: `<domain>_<purpose>_<YYYYMMDD>.yaml`
- Status sidecar file: `<task-file-basename>.status.yaml`
- Team/template status sidecar: `<plan_id>.status.yaml`

### 5.3 Scripts

- Check scripts: `check-<topic>.sh`
- Domain scripts: `<domain>/<verb>-<target>.sh`

Status suffix policy:

- The only approved runtime status suffix is `.status.yaml`.
- Do not introduce parallel suffixes such as `.progress.yaml`, `.state.yaml`, or `.status.json`.

## 7. Reuse-First Catalog (Do Not Re-Implement)

| Capability | Reuse entrypoint | Do not build |
| --- | --- | --- |
| CLI wrappers | `scripts/cli/**` | parallel wrappers in other folders |
| Task dispatch tool | `internal/infra/tools/builtin/orchestration/run_tasks.go` | `run_jobs`, `dispatch_v2`, etc. |
| Task file protocol | `internal/domain/agent/taskfile/**` | new task DSL/parser |
| Status sidecar protocol | `internal/domain/agent/taskfile/status.go` | alternate sidecar schema |
| Team role/stage channel | `internal/domain/agent/ports/agent/team.go`, `internal/domain/agent/taskfile/template.go` | separate role orchestration schema |
| tmux/process management | `internal/devops/process/manager.go`, `internal/infra/process/**`, `cmd/alex/dev.go` | second lifecycle manager |
| External config injection | `internal/shared/config/runtime_env_loader.go`, `internal/infra/process/envmerge.go` | ad-hoc env parsing in business packages |

## 8. New Component Placement Rules

1. If it extends orchestration behavior, modify `taskfile` and `run_tasks`; do not create a new orchestration tool.
2. If it extends role/stage logic, modify team definition/template mapping; do not fork schema.
3. If it extends process lifecycle, modify the existing process manager/controller path.
4. If it adds config input, extend shared config loading and documented config contracts.

## 9. Interface Change Flow

1. Update canonical reference doc first (`docs/reference/**`).
2. Update implementation files (`internal/**`, `cmd/**`).
3. Update tests in matching layers.
4. Update how-to docs (`docs/guides/**`) for operator workflow changes.
5. Keep record docs (`docs/plans/**`, `docs/reviews/**`) as historical context, not canonical truth.

## 10. Review Gate (Anti-Duplication)

Mandatory checks before adding a file/component:

1. Search existing capability paths by keyword and path index.
2. If existing capability matches, reuse or extend directly.
3. If no capability matches, document explicit replacement reason.

Required review fields:

- `Existing capability checked: <path list>`
- `Reuse decision: reuse | extend | new`
- `If new, why not reusable: <single concrete mismatch>`
- `Convergence plan: <future merge path or none>`

Blocking conditions:

1. New parallel orchestration tool when `run_tasks` is sufficient.
2. New status sidecar protocol not ending in `.status.yaml`.
3. New business-layer env parser bypassing shared config loader.
4. New tmux/PID lifecycle manager outside existing process management.
5. File placement violating the file-type-to-directory mapping.

## 11. Discoverability Commands

Use these commands before implementation:

```bash
rg -n "run_tasks|taskfile|status\\.yaml|TeamDefinition|tmux|process manager|runtime_env_loader|envmerge|wrapper" internal scripts docs configs
rg --files internal/domain/agent/taskfile internal/infra/tools/builtin/orchestration internal/infra/process internal/devops/process scripts/cli
```
