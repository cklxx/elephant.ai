# ALEX Web Frontend - Structure

High-level overview of the `web/` workspace. For an exact file list, run `git ls-files web`.

## Top-Level Directories

- `app/`: Next.js App Router routes (server + client components).
- `components/`: UI building blocks and agent/session renderers.
- `hooks/`: React hooks (SSE, stores, view helpers).
- `lib/`: API/auth clients, event pipeline/bus, shared types/utilities.
- `docs/`: frontend design notes (event stream, component architecture, performance).
- `e2e/`: Playwright end-to-end tests.
- `tests/`: unit/integration test helpers and fixtures.
- `.storybook/`: Storybook configuration.

## Conventions

- Put non-React logic in `lib/` first; wrap it for components via `hooks/` and `components/`.
- Keep reusable primitives in `components/ui/`; keep agent-specific UI in `components/agent/`.
- Prefer named exports for UI components and keep files focused (one major component per file).

