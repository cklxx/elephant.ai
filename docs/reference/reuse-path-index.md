# Reuse Path Index

Updated: 2026-03-04

This index is used during precheck to find existing capabilities before adding files.

## Capability Index

| Capability | Search keywords | Canonical paths | Reuse notes |
| --- | --- | --- | --- |
| Task dispatch | `run_tasks`, `orchestration` | `internal/infra/tools/builtin/orchestration/run_tasks.go` | Always extend this tool; no parallel dispatcher |
| Task file schema/validation | `taskfile`, `validate`, `topo` | `internal/domain/agent/taskfile/` | Single task DSL source |
| Task status sidecar | `StatusWriter`, `ReadStatusFile`, `status.yaml` | `internal/domain/agent/taskfile/status.go` | Keep `.status.yaml` protocol |
| Team templates and role channels | `TeamDefinition`, `TeamTemplate`, `stages`, `roles` | `internal/domain/agent/ports/agent/team.go`, `internal/domain/agent/taskfile/template.go` | Keep role/stage model unified |
| Process lifecycle and tmux | `process manager`, `tmux`, `PID`, `attach`, `capture` | `internal/devops/process/manager.go`, `internal/infra/process/`, `cmd/alex/dev.go` | Reuse existing lifecycle manager |
| External agent bridges | `bridge`, `codex_bridge`, `cc_bridge`, `kimi_bridge` | `internal/infra/external/bridge/`, `scripts/codex_bridge/`, `scripts/cc_bridge/`, `scripts/kimi_bridge/` | Reuse bridge protocol, do not fork |
| Config env injection | `runtime_env_loader`, `envmerge` | `internal/shared/config/runtime_env_loader.go`, `internal/infra/process/envmerge.go` | Shared config pipeline only |
| CLI wrappers | `scripts/cli`, `wrapper` | `scripts/cli/` | Put wrapper scripts here only |
| Tool registration | `toolregistry`, `RegisterOrchestration` | `internal/app/toolregistry/registry.go` | Register in existing registry flow |

## Internal Placement Routes

| Responsibility keyword | Place under | Primary anchors |
| --- | --- | --- |
| `use-case`, `coordinator`, `execution flow`, `DI composition` | `internal/app/**` | `internal/app/agent/`, `internal/app/di/` |
| `entity`, `domain rule`, `port`, `state transition` | `internal/domain/**` | `internal/domain/agent/`, `internal/domain/task/`, `internal/domain/workflow/` |
| `adapter`, `provider client`, `bridge`, `tool impl`, `storage impl` | `internal/infra/**` | `internal/infra/tools/`, `internal/infra/external/`, `internal/infra/process/` |
| `http handler`, `sse`, `channel gateway`, `formatter` | `internal/delivery/**` | `internal/delivery/server/`, `internal/delivery/channels/` |
| `config`, `logging`, `json`, `generic utility` | `internal/shared/**` | `internal/shared/config/`, `internal/shared/logging/`, `internal/shared/json/` |
| `local supervisor`, `dev process lifecycle` | `internal/devops/**` | `internal/devops/process/`, `internal/devops/supervisor/` |
| `test helper`, `fixture util` | `internal/testutil/**` | `internal/testutil/` |

## Quick Lookup Commands

```bash
# 1) broad capability scan
rg -n "run_tasks|taskfile|status\\.yaml|TeamDefinition|tmux|runtime_env_loader|envmerge|wrapper" internal scripts docs configs

# 2) orchestration-only
rg -n "run_tasks|reply_agent|taskfile|StatusWriter|TeamTemplate" internal/domain/agent internal/infra/tools/builtin/orchestration

# 3) process management-only
rg -n "tmux|PID|process manager|attach|capture|StartTmux" cmd internal/devops internal/infra/process

# 4) config injection-only
rg -n "runtime_env_loader|MergeEnv|external_agents|env" internal/shared/config internal/infra/process
```

## Precheck Decision Rule

1. If a capability path exists in this table, reuse or extend there.
2. If it does not exist, provide explicit mismatch evidence in plan/PR notes.
3. New file creation without path-index lookup is non-compliant.
