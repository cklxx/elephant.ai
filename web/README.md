# ALEX Web UI

Next.js App Router frontend for the ALEX AI programming agent. The UI streams task events over SSE, renders tool calls and artifacts, and manages sessions for long-running work.

## Highlights

- Real-time SSE event stream with reconnect and backoff
- Task execution with live progress and timeline rendering
- Session history with fork and delete actions
- Tool call cards, markdown rendering, and attachment/skills panels
- Type-safe React + TypeScript with Tailwind CSS styling

## Requirements

- Node.js 20+ and npm
- ALEX backend server running (see repo root `README.md`)

## Quick Start

```bash
npm install
cp .env.local.example .env.local
```

Edit `.env.local` and set at least:

```env
NEXT_PUBLIC_API_URL=http://localhost:8080
```

Start the dev server:

```bash
npm run dev
```

Notes:
- `npm run dev/build/test` automatically regenerates `web/lib/generated/skillsCatalog.json` from the repo-level `skills/` folder via `web/scripts/generate-skills-catalog.js`.
- The dev script chooses an open port starting from `3000`. Set `PORT=...` or `DISABLE_TURBO=1` if needed.

## Environment Variables

Core:
- `NEXT_PUBLIC_API_URL`: API base URL. Use `auto` to resolve from `window.location` (default). Falls back to `http://localhost:8080` in dev and `http://alex-server:8080` in production.

Analytics (optional):
- `NEXT_PUBLIC_POSTHOG_KEY`: PostHog API key (enables analytics).
- `NEXT_PUBLIC_POSTHOG_HOST`: PostHog host (defaults to `https://app.posthog.com`).

Hosting (optional):
- `NEXT_PUBLIC_BASE_PATH`: Next.js base path for subpath deployments.
- `NEXT_PUBLIC_ASSET_PREFIX`: Asset prefix for CDN or subpath hosting.

Debugging (optional):
- `NEXT_PUBLIC_DEBUG_UI=1` to force debug UI.
- Or set `?debug=1` in the URL or `localStorage.alex_debug=1`.

## Common Scripts

```bash
npm run dev           # start dev server (predev generates skills catalog)
npm run build         # production build
npm run start         # run production server
npm run lint          # eslint
npm run test          # vitest run
npm run test:watch    # vitest watch mode
npm run test:coverage # vitest coverage
npm run e2e           # playwright tests
npm run storybook     # storybook dev server
```

## Project Layout

```
web/
├── app/            # App Router routes, layouts, and pages
├── components/     # UI and agent-facing components
├── hooks/          # SSE, tasks, and session hooks
├── lib/            # API client, types, utilities, analytics
├── scripts/        # dev tooling (skills catalog, dev server)
├── tests/          # test helpers and fixtures
├── e2e/            # Playwright tests
└── docs/           # frontend-specific docs
```

## Data Flow (High Level)

- Create a task with `POST /api/tasks` (React Query mutation).
- Open the SSE stream at `/api/sse?session_id=...` (optionally `&replay=none|session|full`).
- Render server events in the conversation timeline.

For a deeper view, read `web/ARCHITECTURE.md`.

## Event Types

Events mirror Go types in `internal/agent/domain/events.go`:

- `workflow.node.started`
- `workflow.node.output.delta`
- `workflow.node.output.summary`
- `workflow.tool.started`
- `workflow.tool.progress`
- `workflow.tool.completed`
- `workflow.node.completed`
- `workflow.result.final`
- `error`

## API Endpoints (Backend)

- `POST /api/tasks` - create and execute a task
- `GET /api/tasks/:id` - task status
- `POST /api/tasks/:id/cancel` - cancel task
- `POST /api/sessions` - create an empty session (for UI prewarm)
- `GET /api/sessions` - list sessions
- `GET /api/sessions/:id` - session details
- `DELETE /api/sessions/:id` - delete session
- `POST /api/sessions/:id/fork` - fork session
- `GET /api/sse?session_id=...` - SSE event stream (`replay=none|session|full`)

## Contributing

Follow the repo-level contribution guidelines.
