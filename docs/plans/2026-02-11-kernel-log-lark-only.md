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

## Verification

- `go build ./...` — pass
- `go test ./internal/app/agent/kernel/... -race` — pass
- `go test ./internal/shared/config/... -run TestNoUnapprovedGetenv` — pass
- `golangci-lint run` — no new warnings
