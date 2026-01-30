# OKR + Scheduler + Proactive Notifications — Implementation Plan

> Started: 2026-01-31
> Status: complete

## Scope
- OKR tools (okr_read, okr_write) with single-file markdown storage
- Proactive OKR context hook
- OKR skill
- Full scheduler (robfig/cron v3) with static + OKR-derived triggers
- Lark notification routing
- Config, registry, DI wiring

## Progress
- [x] Codebase exploration complete
- [x] Memory loaded
- [x] A.1: OKR foundation (types, config, store, tests) — 14 tests
- [x] A.1: OKR tools (okr_read, okr_write, tests) — 13 tests
- [x] A.2: OKR hook (hooks.go constant, okr_context.go, tests) — 7 tests
- [x] A.3: OKR skill (SKILL.md)
- [x] C: OKR registration (config, registry, DI wiring)
- [x] B.1: Add robfig/cron/v3 dependency
- [x] B.2: Scheduler foundation (scheduler, trigger, executor, tests) — 12 tests
- [x] B.3: Scheduler notifier (interface, Lark impl)
- [x] B.4: Scheduler integration (server bootstrap)
- [x] Lint + test validation (all 53 new tests pass, lint clean)

## Commits
1. `4513b98a` — feat(okr): add OKR foundation — types, config, store with tests
2. `51021f03` — feat(okr): add okr_read and okr_write tools with tests
3. `92150d9f` — feat(hooks): add OKR context injection hook
4. `e3a231b8` — feat(skills): add OKR management skill
5. `cf992c7d` — feat(okr): wire OKR config, tool registration, and DI hooks
6. `32de4d9d` — feat(scheduler): add full cron scheduler with OKR trigger sync
7. `c0d1290b` — feat(scheduler): add Lark notifier for scheduler result delivery
8. `aeb705f9` — feat(scheduler): integrate scheduler into server bootstrap

## Files Created/Modified
| File | Action |
|------|--------|
| `internal/tools/builtin/okr/types.go` | CREATE |
| `internal/tools/builtin/okr/config.go` | CREATE |
| `internal/tools/builtin/okr/store.go` | CREATE |
| `internal/tools/builtin/okr/store_test.go` | CREATE |
| `internal/tools/builtin/okr/okr_read.go` | CREATE |
| `internal/tools/builtin/okr/okr_read_test.go` | CREATE |
| `internal/tools/builtin/okr/okr_write.go` | CREATE |
| `internal/tools/builtin/okr/okr_write_test.go` | CREATE |
| `internal/agent/app/hooks/hooks.go` | EDIT — add InjectionOKRContext |
| `internal/agent/app/hooks/okr_context.go` | CREATE |
| `internal/agent/app/hooks/okr_context_test.go` | CREATE |
| `skills/okr-management/SKILL.md` | CREATE |
| `internal/config/types.go` | EDIT — add OKRProactiveConfig, ChatID |
| `internal/toolregistry/registry.go` | EDIT — add OKRGoalsRoot, register tools |
| `internal/di/container_builder.go` | EDIT — wire OKR hook, pass GoalsRoot |
| `internal/scheduler/trigger.go` | CREATE |
| `internal/scheduler/executor.go` | CREATE |
| `internal/scheduler/scheduler.go` | CREATE |
| `internal/scheduler/scheduler_test.go` | CREATE |
| `internal/scheduler/notifier.go` | CREATE |
| `internal/server/bootstrap/server.go` | EDIT — start scheduler |
| `internal/server/bootstrap/scheduler.go` | CREATE |
| `go.mod` / `go.sum` | EDIT — add robfig/cron/v3 |

## Key Patterns Used
- Tools: implement `ToolExecutor` (Execute, Definition, Metadata)
- Hooks: implement `ProactiveHook` (Name, OnTaskStart, OnTaskCompleted)
- Registration: `r.static["name"] = ...` in `registerBuiltins`
- DI: `buildHookRegistry` for hooks, `buildToolRegistry` for tools
- Lark messages: `larkim.NewCreateMessageReqBuilder()` with `chat_id`
- Config: YAML tags on structs in `internal/config/types.go`
- Module: `alex` (go.mod module name)

## Decisions
- OKR files stored as markdown with YAML frontmatter in `goals/` dir
- Default goals root: `~/.alex/goals`
- YAML frontmatter parsing via `gopkg.in/yaml.v3`
- Scheduler uses `robfig/cron/v3` with minute-level parser (5-field cron)
- SchedulerTriggerConfig gets `ChatID` field added
- OKR-derived triggers auto-synced every 5 min from active goal files
- Scheduler Stop() is idempotent via sync.Once
