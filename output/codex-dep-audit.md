# Dependency Audit

Date: 2026-03-10

## Commands Run

```bash
go list -m -u all
govulncheck ./...
GOTOOLCHAIN=go1.24.13 govulncheck -show verbose ./...
```

## Executive Summary

- The repository has multiple available dependency updates, including several direct dependencies and the Go toolchain itself.
- `govulncheck` found 3 reachable vulnerabilities in the Go standard library when scanned under the repo's declared toolchain `go1.24.13`.
- `govulncheck` also found 3 module-level advisories in `golang.org/x/crypto`, which is force-pinned via `replace` to `v0.39.0`.
- No dependency changes were made in this task.

## Highest-Priority Findings

### 1. Reachable standard-library vulnerabilities under the pinned toolchain

The repo declares:

```go
go 1.24.0
toolchain go1.24.13
```

Under `GOTOOLCHAIN=go1.24.13`, `govulncheck` reported these reachable issues:

| Vulnerability | Package | Found | Fixed | Example Reachability |
|---|---|---|---|---|
| `GO-2026-4603` | `html/template` | `go1.24.13` | `go1.25.8` | `tests/utils/report-generator.go`, `internal/delivery/eval/bootstrap/server.go` |
| `GO-2026-4602` | `os` | `go1.24.13` | `go1.25.8` | `internal/delivery/output/cli_renderer_markdown.go`, `internal/app/scheduler/scheduler.go` |
| `GO-2026-4601` | `net/url` | `go1.24.13` | `go1.25.8` | `internal/infra/lark/oauth/service.go`, `internal/app/notification/notification.go` |

Assessment:

- This is the most important dependency finding because the vulnerable code is in the active call graph.
- The durable fix is a Go toolchain upgrade to a version that includes the standard-library fixes.

### 2. `golang.org/x/crypto` is force-pinned to a vulnerable version

`go.mod` contains:

```go
replace golang.org/x/crypto => golang.org/x/crypto v0.39.0
```

`govulncheck -show verbose` reported these module advisories:

| Vulnerability | Module Version | Fixed In | Notes |
|---|---|---|---|
| `GO-2025-4135` | `golang.org/x/crypto@v0.39.0` | `v0.45.0` | `ssh/agent` DoS |
| `GO-2025-4134` | `golang.org/x/crypto@v0.39.0` | `v0.45.0` | `ssh` memory-consumption DoS |
| `GO-2025-4116` | `golang.org/x/crypto@v0.39.0` | `v0.43.0` | `ssh/agent` DoS |

Assessment:

- `govulncheck` did not find these vulnerabilities on a reachable package/symbol path in this repo.
- The explicit `replace` keeps the project below the fixed versions even though the module graph can otherwise see newer releases.
- This should be reviewed before any future dependency refresh, because the override may no longer be justified.

## Direct Dependencies With Available Updates

The following direct dependencies have newer versions available:

| Module | Current | Latest Seen |
|---|---|---|
| `github.com/alecthomas/chroma/v2` | `v2.14.0` | `v2.23.1` |
| `github.com/charmbracelet/x/ansi` | `v0.10.1` | `v0.11.6` |
| `github.com/goccy/go-json` | `v0.10.3` | `v0.10.5` |
| `github.com/mattn/go-sqlite3` | `v1.14.33` | `v1.14.34` |
| `github.com/minio/minio-go/v7` | `v7.0.73` | `v7.0.99` |
| `github.com/mymmrac/telego` | `v1.0.2` | `v1.7.0` |
| `github.com/posthog/posthog-go` | `v1.6.12` | `v1.10.0` |
| `go.opentelemetry.io/otel` | `v1.40.0` | `v1.42.0` |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp` | `v1.40.0` | `v1.42.0` |
| `go.opentelemetry.io/otel/exporters/prometheus` | `v0.62.0` | `v0.64.0` |
| `go.opentelemetry.io/otel/metric` | `v1.40.0` | `v1.42.0` |
| `go.opentelemetry.io/otel/sdk` | `v1.40.0` | `v1.42.0` |
| `go.opentelemetry.io/otel/sdk/metric` | `v1.40.0` | `v1.42.0` |
| `go.opentelemetry.io/otel/trace` | `v1.40.0` | `v1.42.0` |
| `golang.org/x/net` | `v0.49.0` | `v0.51.0` |
| `golang.org/x/term` | `v0.39.0` | `v0.40.0` |
| `golang.org/x/time` | `v0.14.0` | `v0.15.0` |

Observations:

- Most direct updates are minor/patch releases.
- `github.com/mymmrac/telego` and `github.com/minio/minio-go/v7` are materially behind and would need compatibility review before updating.
- The OpenTelemetry stack is consistently two minor releases behind; if updated, those modules should move together.

## Tooling Notes

- `govulncheck ./...` initially failed because the installed `govulncheck` binary was built against Go 1.24 internals while `go` on `PATH` is `go1.26.0`.
- Re-running with `GOTOOLCHAIN=go1.24.13` produced a valid vulnerability scan aligned with the repo's declared toolchain.

## Recommended Next Actions

1. Upgrade the Go toolchain to a release containing the fixes for `GO-2026-4601`, `GO-2026-4602`, and `GO-2026-4603`.
2. Re-evaluate the `replace golang.org/x/crypto => ... v0.39.0` override and remove or raise it to at least the fixed range if no longer needed.
3. Batch direct dependency updates by subsystem rather than doing a repo-wide bump in one change set.
