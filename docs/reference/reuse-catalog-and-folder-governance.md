# Reuse Catalog And Folder Governance

Updated: 2026-03-10

Use this doc before adding files or components.

## Core Rules

1. Reuse or extend existing code first.
2. Put every new file in its canonical directory.
3. Runtime status sidecars use only `.status.yaml`.
4. Configuration examples are YAML-only.
5. If reuse is impossible, record the exact path and contract mismatch.

## File Placement

| File type | Put it here | Do not put it here |
| --- | --- | --- |
| Runtime Go code | `internal/**` | `docs/**`, `tasks/**`, repo root |
| Go entrypoints | `cmd/**` | `internal/**` |
| Unit tests | next to the package | unrelated test folders |
| Cross-module integration tests | `tests/**` or existing integration test dirs | `docs/**` |
| Runtime config YAML | `configs/**` | `internal/**`, `scripts/**` |
| Task input YAML | `tasks/**` | `configs/**`, `docs/**` |
| Status sidecars | `tasks/**` or `.elephant/tasks/**` | anywhere else |
| Canonical references | `docs/reference/**` | `docs/plans/**` |
| Guides | `docs/guides/**` | `docs/reference/**` when the doc is procedural only |
| Plans and progress | `docs/plans/**` | `docs/reference/**` |
| Reviews, research, analysis | `docs/reviews/**`, `docs/research/**`, `docs/analysis/**` | `docs/reference/**` |
| Shell and helper scripts | `scripts/**` | repo root |
| Runtime artifacts | `logs/**`, `pids/**`, `tmp/**`, `.tmp/**`, `artifacts/**` | source or docs folders |

## `internal/` Placement

Every file under `internal/` belongs to one existing first-level namespace.

| Path | Responsibility |
| --- | --- |
| `internal/domain/` | business rules, entities, domain ports, domain events |
| `internal/app/` | use cases, orchestration, DI-facing coordination |
| `internal/infra/` | concrete adapters to external systems |
| `internal/delivery/` | HTTP, SSE, channel gateways, output formatting |
| `internal/shared/` | generic cross-cutting utilities |
| `internal/devops/` | local runtime/process orchestration |
| `internal/testutil/` | reusable test-only helpers |

Do not add a new first-level namespace under `internal/` without an architecture plan.

## Reuse-First Anchors

| Capability | Canonical path |
| --- | --- |
| Team CLI surface | `cmd/alex/team_cmd.go` |
| Internal team runner | `internal/infra/tools/builtin/orchestration/team_runner.go` |
| Taskfile schema and dispatch | `internal/domain/agent/taskfile/` |
| Status sidecar protocol | `internal/domain/agent/taskfile/status.go` |
| Team template mapping | `internal/domain/agent/taskfile/template.go` |
| Process lifecycle | `internal/devops/process/manager.go`, `internal/infra/process/` |
| Config env injection | `internal/shared/config/runtime_env_loader.go`, `internal/infra/process/envmerge.go` |
| Tool registration | `internal/app/toolregistry/registry.go` |

## Pre-Check

Before adding a file:

1. Classify the file by type and responsibility.
2. Resolve the target directory from this doc.
3. Search `docs/reference/reuse-path-index.md` and the codebase.
4. Choose one action: `reuse`, `extend`, or `new`.
5. If the action is `new`, record why existing paths are not reusable.

## Blocking Conditions

The change is blocked if it:

- adds a parallel orchestration path instead of extending the existing one
- introduces a status format other than `.status.yaml`
- bypasses the shared config/env-loading path
- puts files in the wrong directory for their responsibility
- changes a contract without updating canonical reference docs
