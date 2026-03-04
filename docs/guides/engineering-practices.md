# Engineering Practices

Updated: 2026-03-03

These are the local engineering practices for this repo. Keep them short and actionable.

See also: [Development Workflow](development-workflow.md) | [Code Simplification](code-simplification.md) | [Code Review](code-review-guide.md) | [Incident Response](incident-response.md) | [Memory Management](memory-management.md)

## Core
- Prefer correctness and maintainability over short-term speed.
- Make small, reviewable changes; avoid large rewrites unless explicitly needed.
- Use TDD when touching logic; include edge cases.
- When users choose multiple numbered options (for example "1 and 2"), implement every selected item in the same delivery unless explicitly constrained.
- Run full lint and tests before delivery.
- On macOS, prefer `CGO_ENABLED=0` for Go tests with `-race` to avoid LC_DYSYMTAB linker warnings; set `CGO_ENABLED=1` when cgo is required.
- Keep config examples in YAML only (no JSON configs).

## Planning & Records
- Every non-trivial task must have a plan file under `docs/plans/` and be updated as work progresses.
- Continuously review best practices and execution flow; record improvements and update guides/plans when new patterns emerge.
- For governance and folder-rule tasks, specify deterministic file-type-to-directory mapping and naming rules; avoid ambiguous high-level wording.
- For `internal/**` governance, always include explicit first-level namespace routing (`app/domain/infra/delivery/shared/devops/testutil`) and forbidden placements.
- Log notable incidents in `docs/error-experience/entries/` and add a summary entry under `docs/error-experience/summary/entries/`.
- Log notable wins in `docs/good-experience/entries/` and add a summary entry under `docs/good-experience/summary/entries/`.
- Keep `docs/error-experience.md` and `docs/error-experience/summary.md` as index-only.
- Keep `docs/good-experience.md` and `docs/good-experience/summary.md` as index-only.

## Safety
- Avoid destructive operations or history rewrites unless explicitly requested.
- Prefer reversible steps and explain risks when needed.
- If command policy blocks `git branch -d/-D`, use Git plumbing fallback: `git update-ref -d refs/heads/<branch>` then `git worktree prune`; verify with `git branch --list '<branch>'`.
- When unrelated modified files are present, explicitly surface them and exclude them from staging/commit unless the user asks to include them.

## Code Style
- Avoid unnecessary defensive code; trust invariants when guaranteed.
- Keep naming consistent; follow local naming guidelines when present.
- Be cautious with long parameter lists; if a function needs many inputs, prefer grouping into a struct or options pattern and document the boundary explicitly.
- Prefer `internal/shared/json` (`jsonx`) for JSON encode/decode hot paths; avoid direct `encoding/json` unless `jsonx` cannot provide the required API.
- Follow the [Code Simplification Best Practices](code-simplification.md) — covers shared utility usage, error handling, struct sizing, caching, I/O patterns, and DRY across providers.

## Go + OSS (Condensed)
- Formatting/imports: always run `gofmt`; use `goimports` to manage imports.
- Naming: package names are lowercase and avoid underscores/dashes; avoid redundant interface names (`storage.Interface` not `storage.StorageInterface`).
- Comments: exported identifiers have full-sentence doc comments that start with the identifier name.
- Context: pass `context.Context` explicitly (first param); never store it in structs.
- Errors: check/handle errors; avoid `panic` for normal flow; wrap with context when returning.
- Concurrency & tests: avoid fire-and-forget goroutines (make lifetimes explicit); prefer table-driven tests for multi-case coverage.

Sources: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md), [Kubernetes Coding Conventions](https://www.kubernetes.dev/docs/guide/coding-convention/).

## Architecture Principles

- **Context engineering over prompt hacking** — modify context assembly (`internal/context/`) first. Prompt templates only if context changes are verified insufficient.
- **Typed events over unstructured logs** — use typed event structs (`internal/agent/domain/events/`). No free-form log strings for state transitions.
- **Clean port/adapter boundaries** — cross-layer imports go through port interfaces. Direct infra-to-domain imports forbidden. Keep `agent/ports` free of memory/RAG deps. Enforce with `make check-arch`.
- **Multi-provider LLM support** — new LLM features must work across all providers in `internal/llm/`. No provider-specific APIs without adapter.
- **Skills and memory over one-shot answers** — persist learnings to memory; encode reusable workflows as skills.
- **Proactive context injection** — auto-inject relevant context before user asks. Manual retrieval is fallback.

## Proactive Behavior Constraints

When modifying proactive behavior code (`internal/agent/`, skill triggers, context injection):
- Detect motivation state before proactive actions: low energy, overload, ambiguity, or clear readiness.
- Minimum-effective intervention: `clarify` → `plan` → reminder/schedule/task execution.
- Every proactive suggestion must remain user-overridable; never remove opt-out paths.
- Prefer progress visibility (artifacts/checkpoints) over high-frequency nudges.
- No manipulative framing (fear, guilt, urgency) in any LLM prompt construction.
- External messages or irreversible operations must pass approval gates. No exceptions.
- Honor stop signals immediately.

## Self-Correction

Upon receiving any correction, immediately write a preventive rule (in `docs/guides/`, `docs/error-experience/entries/`, or the relevant best-practice doc) to prevent the same class of mistake. Do not wait — codify the lesson before resuming work.
