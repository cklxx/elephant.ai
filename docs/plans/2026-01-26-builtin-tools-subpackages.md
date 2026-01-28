# Plan: Split builtin tools into domain subpackages (Phase 3) (2026-01-26)

## Goal
- Split `internal/tools/builtin` into domain-focused subpackages and adjust registry wiring with minimal diff blast radius.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Current state (systematic view)
- `internal/tools/` contains a single `builtin` package with ~80 files across file ops, search, execution, sandbox, web, artifacts, media, orchestration, and UI helpers.
- `internal/toolregistry/registerBuiltins` wires all builtins via `alex/internal/tools/builtin`.
- Non-registry callers import `builtin` for context helpers (`WithApprover`, `WithAutoApprove`, `WithToolSessionID`, `WithWorkingDir`, `WithParentListener`) and `SubtaskEvent`.
- Several helper utilities (path resolver/guard, context keys, validation, tool helpers) are shared across multiple tool domains.

## Proposed package layout

### Shared utilities (no tool implementations)
- `internal/tools/builtin/shared`
  - Context helpers: approver/auto-approve/tool session/working dir/allow-local-fetch/session helpers.
  - `tool_helpers.go`, `validation.go`, `types.go` (or remove if unused after move).
- `internal/tools/builtin/pathutil`
  - `path_guard.go`, `path_resolver.go` (used by file, search, exec).

### Tool domains (tool implementations + tests)
- `internal/tools/builtin/fileops`
  - `file_read.go`, `file_write.go`, `file_edit.go`, `list_files.go`.
- `internal/tools/builtin/search`
  - `grep.go`, `ripgrep.go`, `find.go`, `code_search.go`, `search_helpers.go`.
- `internal/tools/builtin/execution`
  - `bash.go`, `bash_local.go`, `code_execute.go`, `code_execute_local.go`, `local_exec_flag_*.go`, `acp_executor*.go`.
- `internal/tools/builtin/session`
  - `todo_read.go`, `todo_update.go`, `skills.go`, `apps.go`.
- `internal/tools/builtin/ui`
  - `plan.go`, `clarify.go`, `request_user.go`, `attention.go`, `think.go`.
- `internal/tools/builtin/memory`
  - `memory_recall.go`, `memory_write.go`.
- `internal/tools/builtin/web`
  - `web_search.go`, `web_fetch.go`, `html_edit.go`, `douyin_hot.go`, `allow_local_fetch.go`.
- `internal/tools/builtin/artifacts`
  - `artifacts.go`, `artifact_manifest.go`, `attachment_resolver.go`, `attachment_uploader.go`, `pptx_from_images.go`, `a2ui.go`.
- `internal/tools/builtin/media`
  - `seedream_*.go`, `music_play.go`, `seedream_helpers.go`, `seedream_common.go`.
- `internal/tools/builtin/sandbox`
  - `sandbox_*.go`, `sandbox_tools.go`, `sandbox_attachments.go`.
- `internal/tools/builtin/orchestration`
  - `subagent.go`, `explore.go`.

### Registry wiring
- Update `internal/toolregistry/registry.go` imports to domain packages (e.g., `fileops.NewFileRead`, `execution.LocalExecEnabled`).

## Steps to minimize diff blast radius
1. **Create shared packages first** (`shared`, `pathutil`) and move helpers; update tool files to import helpers.
2. **Move low-coupling domains first** (media, sandbox, artifacts) to reduce cross-package churn; update tool registry wiring incrementally.
3. **Move file/search/exec domains next**; keep build-tag files (`local_exec_flag_*.go`) with execution package to avoid build tag surprises.
4. **Move UI/session/orchestration last**, then update non-registry callers (CLI/server) to new import paths for `SubtaskEvent` and context helpers.
5. **Run gofmt/goimports after each domain move** and run unit tests for the affected packages to catch import cycles early.
6. **Finish with full lint + test** once all packages and registry wiring are updated.

## Risks & mitigations
- **Import cycles**: avoid by isolating shared helpers in `shared`/`pathutil` and keeping each domain self-contained.
- **Build tag drift**: keep `local_exec_flag_*.go` together with `execution` to preserve build behavior.
- **Non-registry imports**: update CLI/server imports last to minimize churn during tool moves.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
- 2026-01-26: Moved builtin tools into domain subpackages (fileops/search/execution/session/ui/web/artifacts/media/sandbox/orchestration/memory) and updated registry wiring + CLI/server imports to use new packages; removed shim approach in favor of direct imports; moved parent listener context helper into `shared`; added `shared.StringMapArg`; exported attachment migration helper and attachment resolver helpers for cross-package reuse.
- 2026-01-26: Consolidated per-domain move notes into this plan and removed redundant per-package plan files to reduce plan sprawl.
