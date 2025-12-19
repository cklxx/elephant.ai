# elephant.ai / ALEX Codebase Readmap

This file is a “read map” (reading order + pointers) for understanding the repository without guesswork.

## 1) Start Here

- `README.md` — product overview, quickstarts, and high-level architecture.
- `docs/README.md` — documentation index.
- `docs/AGENT.md` — the Think→Act→Observe loop, event model, and orchestration flow.

## 2) Delivery Surfaces

- CLI/TUI: `cmd/alex`, `internal/cli`
- Server (HTTP + SSE): `cmd/alex-server`, `internal/server`, `internal/http`
- Web dashboard: `web/` (see `web/README.md`)

## 3) Agent Core (Go)

- Application layer: `internal/agent/app`
- Domain model + ports: `internal/agent/domain`, `internal/agent/ports`
- DI container / wiring: `internal/di`

## 4) Tooling & Context

- Tool registry + builtins: `internal/toolregistry`, `internal/tools`
- Skills (Markdown playbooks): `skills/`
  - Tool access: `internal/tools/builtin/skills.go`
  - Index generation for prompts: `internal/skills/index.go`
  - Prompt injection: `internal/context/manager.go`
- Context builder: `internal/context`

## 5) Web Event Stream (Next.js)

- Event ingestion + de-dupe: `web/hooks/useSSE.ts`
- Attachment hydration: `web/lib/events/attachmentRegistry.ts`
- Main conversation stream UI: `web/components/agent/TerminalOutput.tsx`
- Right resources panel (skills + attachments): `web/app/conversation/ConversationPageContent.tsx`
- Skills catalog generation: `web/scripts/generate-skills-catalog.js` → `web/lib/generated/skillsCatalog.json`

## 6) Tests & Validation

- Go: `make test`, plus targeted packages under `internal/`
- Web unit tests: `(cd web && npm test)`
- Web e2e: `(cd web && npm run e2e)`
- Evaluations: `evaluation/`
