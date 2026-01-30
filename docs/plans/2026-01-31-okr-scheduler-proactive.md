# OKR + Scheduler + Proactive Notifications — Implementation Plan

> Started: 2026-01-31
> Status: in-progress

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
- [ ] A.1: OKR foundation (types, config, store, tests)
- [ ] A.1: OKR tools (okr_read, okr_write, tests)
- [ ] A.2: OKR hook (hooks.go constant, okr_context.go, tests)
- [ ] A.3: OKR skill (SKILL.md)
- [ ] C: OKR registration (config, registry, DI wiring)
- [ ] B.1: Add robfig/cron/v3 dependency
- [ ] B.2: Scheduler foundation (scheduler, trigger, executor, tests)
- [ ] B.3: Scheduler notifier (interface, Lark impl)
- [ ] B.4: Scheduler integration (server bootstrap)
- [ ] Lint + test validation

## Key Patterns Observed
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
- Scheduler uses `robfig/cron/v3` with second-optional parser
- SchedulerTriggerConfig gets `ChatID` field added
- OKR-derived triggers auto-synced every 5 min from active goal files
