# Long-Term Memory

Updated: 2026-03-09 12:00

## Criteria
- Only keep durable knowledge that should persist across tasks.
- Prefer short, actionable statements with a clear remediation or rule.

## Topic Files
- **Eval & Routing** → see [eval-routing.md](eval-routing.md) (suite design, heuristic rules, routing patterns)
- **Kernel Operations** → see [kernel-ops.md](kernel-ops.md) (execution rules, supervisor, process management)
- **Lark & DevOps** → see [lark-devops.md](lark-devops.md) (local ops, PID management, auth infra)
- **Runtime & Events** → see [runtime-events.md](runtime-events.md) (event partitioning, streaming perf, subagent rules)

## Active Rules
- Keep `agent/ports` free of memory/RAG deps; inject memory at engine/app layers.
- Config: YAML-only. Plans in `docs/plans/`. Experience entries/summaries in their respective dirs; index files are index-only.
- TDD when touching logic; `alex dev lint` + `alex dev test` before delivery.
- `CGO_ENABLED=0` for `go test -race` on darwin CLT.
- Prefer `internal/shared/json` (`jsonx`) over `encoding/json` on hot paths.
- Keep `make check-arch` green for domain import boundaries.
- `scripts/pre-push.sh` mirrors CI fast-fail; always before `git push`. Skip: `SKIP_PRE_PUSH=1`.
- Skills resolution: `ALEX_SKILLS_DIR` overrides all; default `~/.alex/skills` with repo `skills/` missing-only sync.
- Memory system: Markdown-only (`~/.alex/memory/MEMORY.md` + daily files).
- Bash `set -u`: guard array expansions to avoid unbound variable errors.
- Subscription model selection: request-scoped, no mutating managed overrides YAML.

## Architecture
- Context engineering over prompt hacking; typed events over unstructured logs.
- Clean port/adapter boundaries; multi-provider LLM support.
- Improvement plan: `docs/plans/architecture-review-2026-02-16.md` — decouple → split god structs → unify events/storage → test coverage.
