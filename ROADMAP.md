# elephant.ai / ALEX Roadmap

This roadmap is a guided reading order for the codebase. Follow it when you onboard, or when you need to trace how an event trave
ls from the CLI/web into the agent core and back to the user.

## 1) Orientation

Start with the "why" and the top-level mechanics:

- `README.md` — product overview, quickstart (`./dev.sh`), and high-level architecture.
- `docs/README.md` — table of contents for deeper docs.
- `docs/AGENT.md` — the Think → Act → Observe loop, event lifecycle, and orchestration semantics.

## 2) Entry Surfaces

Trace how requests arrive and responses are streamed back:

- CLI/TUI: `cmd/alex`, wired through `internal/cli` for prompts, history, and output formatting.
- HTTP + SSE server: `cmd/alex-server`, `internal/server`, `internal/http` for routing and streaming.
- Web dashboard: `web/` (see `web/README.md`) for Next.js app structure and dev tasks.

## 3) Agent Runtime (Go)

Core execution path and orchestration hooks:

- Application services: `internal/agent/app` coordinates conversations and tool calls.
- Domain model + ports: `internal/agent/domain`, `internal/agent/ports` define aggregates, events, and boundaries.
- Dependency wiring: `internal/di` assembles adapters (LLM, vector stores, tools, storage).

## 4) Context, Tools, and Skills

How the agent gathers context and safely executes actions:

- Context builder + prompt injection: `internal/context`, `internal/context/manager.go`.
- Tool registry and built-ins: `internal/toolregistry`, `internal/tools`.
- Skills (Markdown playbooks): `skills/` with LLM-exposed wrappers in `internal/tools/builtin/skills.go`.
- Skill catalog generation: `internal/skills/index.go` produces metadata consumed by the web app.

## 5) Data Plane and Events (Web)

Follow the event stream that powers the dashboard:

- SSE ingestion and dedupe: `web/hooks/useSSE.ts`.
- Attachment hydration + renderers: `web/lib/events/attachmentRegistry.ts`.
- Conversation stream UI: `web/components/agent/ConversationEventStream.tsx`.
- Right-hand resources (skills, attachments): `web/app/conversation/ConversationPageContent.tsx`.
- Catalog generation bridge: `web/scripts/generate-skills-catalog.js` → `web/lib/generated/skillsCatalog.json`.

## 6) Persistence and Ops

Supporting infrastructure and migrations:

- Database migrations: `migrations/` and `deploy/` manifests for cluster installs.
- Configuration: `configs/` and service defaults under `internal/config`.
- Deployment helpers: `deploy.sh`, `k8s/`, and Docker images built via `Makefile` targets.

## 7) Quality Gates

Run these to validate changes end-to-end:

- Go lint + tests: `./scripts/run-golangci-lint.sh run --timeout=10m ./...` and `make test`.
- Web lint + unit tests: `npm --prefix web run lint` and `npm --prefix web test`.
- End-to-end + evaluations: `npm --prefix web run e2e` and `./dev.sh test` for the orchestrated suite.
