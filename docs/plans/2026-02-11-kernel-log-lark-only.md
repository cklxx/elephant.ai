# Kernel Dedicated Log + Lark-Only Startup

Created: 2026-02-11
Status: Implemented

---

## Problem

1. Kernel agent loop logs are mixed into `alex-service.log`, making it hard to observe kernel behavior independently.
2. Kernel starts in both `alex-server` (HTTP mode) and `alex-server lark`, but only Lark mode has the gateway to deliver REPORT messages.

## Process Separation Analysis

### Conclusion: V1 stays in-process

- Kernel is a lightweight periodic task (`*/10` cron), not a high-throughput service
- It depends on AgentCoordinator and full tool registry — separating would duplicate the entire DI container
- Dedicated log file (`alex-kernel.log`) provides sufficient observability
- If kernel becomes compute-intensive in the future, extract to a separate process with gRPC/message queue

See full analysis in the plan transcript.

## Changes

| File | Change |
|------|--------|
| `internal/shared/utils/logger.go` | Added `LogCategoryKernel` + `logFileName` case for `alex-kernel.log` |
| `internal/shared/logging/logger.go` | Added `NewKernelLogger()` |
| `internal/app/di/container_builder.go` | `buildKernelEngine` uses `NewKernelLogger("KernelEngine")` |
| `internal/infra/kernel/postgres_store.go` | Uses `NewKernelLogger("KernelDispatchStore")` |
| `internal/delivery/server/bootstrap/kernel.go` | Uses `NewKernelLogger("KernelStage")` for bootstrap logs |
| `internal/delivery/server/bootstrap/server.go` | Removed `KernelStage` from HTTP mode gateway stages |

## Config Simplification (batch 2)

### Problems addressed
1. `ProactiveFileConfig` had no `Kernel` field — kernel config could not be loaded from YAML
2. `kernel.KernelConfig` (12 fields) mirrored `KernelProactiveConfig` — DI builder did field-by-field copy of 6 unused fields
3. `configs/config.yaml` had no `proactive.kernel` section — kernel could never actually run

### Changes

| File | Change |
|------|--------|
| `internal/app/agent/kernel/config.go` | Trimmed `KernelConfig` from 12 to 6 fields (Engine-consumed only) |
| `internal/shared/config/file_config.go` | Added `KernelFileConfig` + `KernelAgentFileConfig` |
| `internal/shared/config/proactive_merge.go` | Added `mergeKernelConfig()` + env expansion + wired into `mergeProactiveConfig` |
| `internal/shared/config/proactive_merge_test.go` | Added merge, nil-safety, proactive-includes, and env expansion tests |
| `internal/app/di/container_builder.go` | Simplified `buildKernelEngine` config literal from 12 to 6 fields |
| `internal/app/agent/kernel/engine_test.go` | Removed unused `TimeoutSeconds` from test config |
| `configs/config.yaml` | Added `proactive.kernel` section with seed state |

## Verification

- `go build ./...` — pass
- `go test ./internal/app/agent/kernel/... -race` — pass
- `go test ./internal/shared/config/... -count=1` — pass (including new merge tests)
- `go test ./internal/shared/config/... -run TestNoUnapprovedGetenv` — pass
- `golangci-lint run` — no new warnings
