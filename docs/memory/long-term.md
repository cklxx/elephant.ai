# Long-Term Memory

Updated: 2026-03-10 18:00

## Keep Here

- Only durable rules that should survive across tasks.
- Prefer short statements with a clear rule or remediation.

## Topic Files

- [eval-routing.md](eval-routing.md) — eval structure and routing patterns
- [lark-devops.md](lark-devops.md) — Lark local ops, PID handling, auth rules
- [runtime-events.md](runtime-events.md) — event partitioning, streaming, subagent rules

## Active Rules

- Keep `agent/ports` free of memory and RAG dependencies; inject memory above the domain layer.
- Config examples are YAML-only. Plans live in `docs/plans/`. Experience indexes stay index-only.
- For logic changes, prefer TDD and run relevant lint and tests before delivery.
- On Darwin, use `CGO_ENABLED=0` for `go test -race` unless cgo is required.
- Prefer `internal/shared/json` (`jsonx`) on hot JSON paths.
- Keep architecture boundary checks green.
- Run `scripts/pre-push.sh` before `git push`.
- Skills resolution: `ALEX_SKILLS_DIR` overrides defaults; otherwise prefer user skills dir with repo fallback.
- Memory storage stays Markdown-first.
- Guard Bash array expansions under `set -u`.
- Subscription model selection is request-scoped; do not mutate managed override YAML in place.

## Architecture Defaults

- Prefer context engineering over prompt hacking.
- Prefer typed events over unstructured logs.
- Keep port and adapter boundaries clean.
- New LLM behavior should not hardcode a single provider path.
