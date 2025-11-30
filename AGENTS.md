# Repository Guidelines

## Project Structure & Module Organization
- `cmd/alex`, `cmd/alex-server`: CLI/TUI and HTTP+SSE entrypoints; build outputs in repo root or `build/`.
- `internal/`: agent application layer, domain logic, ports, DI, tooling, observability; keep domain packages infra-free.
- `web/`: Next.js 14 dashboard (TypeScript + Tailwind) with `app/` routes, `components/`, `hooks/`, `lib/`.
- `docs/`: architecture/reference/ops notes; `proto/` protobufs; `examples/` presets; `third_party/` vendored deps.
- `configs/`, `migrations/`, `k8s/`, `nginx.conf` for deployment; `scripts/` automation; `tests/` acceptance harness.

## Build, Test, and Development Commands
- Go toolchain pinned via `scripts/go-with-toolchain.sh` (Go 1.24). `make dev` runs fmt → vet → build.
- Build/run: `make build` then `./alex`; `make server-build` then `./alex-server`.
- Go tests: `make test` (`go test ./...` with race/cover in CI), `make test-domain`, `make test-app` for targeted loops.
- Lint/format: `make fmt` (golangci-lint --fix), `make vet`, `make check-deps` to verify domain purity.
- Frontend: `cd web && npm install`; `npm run dev`; production `npm run build && npm start`; quality gates `npm run lint`, `npm run test`, `npm run e2e` for UI flows.

## Coding Style & Naming Conventions
- Go: gofmt/goimports via `make fmt`; package names lowercase without underscores; exported symbols need GoDoc comments.
- Respect boundaries: domain uses ports only; place adapters under `internal/tools`, `internal/llm`, etc.
- TypeScript/React: functional components and hooks, 2-space indent, folder-kebab routes under `web/app/`; co-locate tests as `*.test.ts(x)`.
- Use descriptive identifiers and reuse config/env names already present in `configs/` and docs.

## UI and Content Guidelines
- Banned: all-caps with forced spacing/letter-tracking (e.g., `C O N T I N U E WITH EMAIL`, `M A R K D O W N`); use normal casing and spacing everywhere.
- Keep interfaces minimal; add new UI elements/entities only when necessary and after careful consideration.

## Testing Guidelines
- Prefer table-driven Go tests next to code (`*_test.go`) with small fixtures and deterministic timing.
- Run `make test` before PRs; add `make test-domain`/`test-app` when touching agent logic. CI also runs golangci-lint, gosec, govulncheck, and lightweight perf checks.
- Frontend updates: `npm run lint` + `npm run test`; run `npm run e2e` for flow or SSE changes; `npm run test:coverage` for local coverage.
- Broader acceptance/perf suites live in `tests/`; coordinate before enabling them in CI.

## Commit & Pull Request Guidelines
- Commit messages stay short and imperative (e.g., `Validate session IDs in filestore`); keep scope tight.
- PR description: intent, key changes, and exact commands executed (e.g., `make test`, `npm run lint`); link issues and call out config/schema changes.
- UI tweaks should include before/after screenshots or CLI output; update docs (`docs/`, `README.md`, inline comments) when behavior shifts.
- Keep dependencies tidy (`go.mod`/`go.sum`, npm lockfiles) and avoid committing build artifacts or local coverage files.
