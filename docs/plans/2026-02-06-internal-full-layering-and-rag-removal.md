# 2026-02-06 Internal Full Layering + RAG Removal

## Goal
- Complete one-pass migration of `internal/` to strict layering under `internal/{delivery,app,domain,infra,shared}`.
- Remove `internal/rag` completely, including config and dependency references.
- Add permanent anti-flyline architecture gates (policy + CI blocking).

## Constraints
- Preserve external semantics for CLI, HTTP/SSE, and Lark interactions.
- Remove only RAG and confirmed orphan code in this round.
- No compatibility shims; imports must be fully rewritten to the new paths.

## Baseline
- Baseline report: `docs/plans/2026-02-06-internal-full-layering-and-rag-removal-baseline.txt`
- Snapshot (before migration):
  - `internal` Go files: `922`
  - Top package concentration: `tools(180)`, `agent(167)`, `server(111)`
  - Largest hotspots: `internal/channels/lark/gateway.go`, `internal/server/http/sse_handler_test.go`, `internal/agent/domain/react/runtime.go`

## Execution Plan
1. Migrate `shared` and `domain` paths first, then rewrite imports.
2. Migrate `app`, then `infra`, then `delivery`.
3. Remove `internal/rag` and strip all RAG config/types/defaults/docs references.
4. Add `configs/arch/{policy,exceptions}.yaml` and `scripts/arch/check-graph.sh`.
5. Wire `make check-arch-policy` into CI blocking path.
6. Run full `lint + test + build` gates.
7. Commit incrementally by phase and merge back to `main` with fast-forward.

## Progress Log
- 2026-02-06 11:12: Created worktree `eli/internal-full-layering-rag-removal` from `main`; copied `.env`.
- 2026-02-06 11:13: Loaded engineering practices + long-term memory context; captured baseline report.
- 2026-02-06 11:18: Completed full path migration to `internal/{delivery,app,domain,infra,shared}` and rewrote imports across Go packages.
- 2026-02-06 11:20: Deleted `internal/rag` sources; removed RAG config structs/merge/default logic; removed `proactive.rag` from `configs/config.yaml`.
- 2026-02-06 11:28: Added architecture governance artifacts (`configs/arch/policy.yaml`, `configs/arch/exceptions.yaml`, `scripts/arch/check-graph.sh`) and CI/Makefile wiring.
- 2026-02-06 11:31: Added `.github/CODEOWNERS` and `.github/PULL_REQUEST_TEMPLATE.md`; updated key architecture/roadmap docs to layered paths.
- 2026-02-06 11:33: Ran `go mod tidy`; removed `chromem-go` dependency; kept `hashicorp/golang-lru/v2` because it is still used by non-RAG packages.
