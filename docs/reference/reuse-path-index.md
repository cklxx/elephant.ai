# Reuse Path Index

Updated: 2026-03-10

Check this index before adding files. Reuse or extend these paths first.

## Capability Index

| Capability | Search keywords | Canonical paths |
| --- | --- | --- |
| Team CLI | `alex team`, `team_cmd`, `template` | `cmd/alex/team_cmd.go` |
| Team runner internals | `team_runner`, `TaskFile`, `dispatch` | `internal/infra/tools/builtin/orchestration/team_runner.go`, `internal/domain/agent/taskfile/` |
| Task status sidecar | `status.yaml`, `StatusWriter`, `ReadStatusFile` | `internal/domain/agent/taskfile/status.go` |
| Team templates and role channels | `TeamDefinition`, `TeamTemplate`, `stages`, `roles` | `internal/domain/agent/ports/agent/team.go`, `internal/domain/agent/taskfile/template.go` |
| Process lifecycle and tmux | `process manager`, `tmux`, `PID`, `attach`, `capture` | `internal/devops/process/manager.go`, `internal/infra/process/`, `cmd/alex/dev.go` |
| External bridges | `bridge`, `codex_bridge`, `cc_bridge`, `kimi_bridge` | `internal/infra/external/bridge/`, `scripts/codex_bridge/`, `scripts/cc_bridge/`, `scripts/kimi_bridge/` |
| Config env injection | `runtime_env_loader`, `envmerge` | `internal/shared/config/runtime_env_loader.go`, `internal/infra/process/envmerge.go` |
| CLI wrappers | `scripts/cli`, `wrapper` | `scripts/cli/` |
| Tool registration | `toolregistry`, `registry` | `internal/app/toolregistry/registry.go` |

## `internal/` Routing

| Need | Place under |
| --- | --- |
| use case, coordinator, execution flow | `internal/app/**` |
| entity, domain rule, port, state transition | `internal/domain/**` |
| adapter, provider client, bridge, storage impl | `internal/infra/**` |
| HTTP handler, SSE, channel gateway, formatter | `internal/delivery/**` |
| config, logging, JSON, generic utility | `internal/shared/**` |
| local supervisor or process lifecycle | `internal/devops/**` |
| test helper or fixture utility | `internal/testutil/**` |

## Quick Commands

```bash
rg -n "team_runner|TaskFile|status\\.yaml|TeamDefinition|tmux|runtime_env_loader|envmerge|wrapper" internal scripts docs configs
rg -n "TeamDefinition|TaskFile|dispatch|template" internal/domain/agent internal/infra/tools/builtin/orchestration cmd/alex
rg -n "tmux|PID|process manager|attach|capture" cmd internal/devops internal/infra/process
rg -n "runtime_env_loader|envmerge|external_agents|env" internal/shared/config internal/infra/process
```

## Decision Rule

1. If a capability already has a path here, reuse or extend it there.
2. If not, record the mismatch in plan or review notes.
3. New file creation without this lookup is non-compliant.
