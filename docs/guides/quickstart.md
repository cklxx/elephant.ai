# Alex / Spinner Quickstart
> Last updated: 2025-12-14

This guide gets you from `git clone` to running the CLI/TUI, server, and web UI with the **single runtime config system** described in `docs/reference/CONFIG.md`.

---

## Prerequisites

- Go **1.24+** (pinned in `go.mod`)
- Node.js **20+** (only needed for the web UI)
- Docker (optional, for Compose/K8s workflows)

---

## Build

```bash
make build         # builds ./alex
make server-build  # builds ./alex-server
```

---

## Configure (single source of truth)

### 1) Main config file

Default location: `~/.alex-config.json` (override via `ALEX_CONFIG_PATH`).

```bash
cp examples/config/core-config-example.json ~/.alex-config.json
```

### 2) Secrets via environment variables (recommended)

```bash
export OPENAI_API_KEY="sk-..."
# Optional tool keys
export TAVILY_API_KEY="..."
export ARK_API_KEY="..."
```

### 3) Optional managed overrides (persistent)

Managed overrides are stored in `~/.alex/runtime-overrides.json` by default (see `alex config path`).

```bash
./alex config
./alex config set llm_model gpt-4o-mini
./alex config set llm_vision_model gpt-4o-mini
./alex config clear llm_vision_model
```

Reference: `docs/reference/CONFIG.md`

---

## Run

### CLI / TUI

```bash
./alex
```

### One-shot task

```bash
./alex "analyze the authentication flow and list risks"
```

### Server

```bash
make server-run
```

### Web UI (development)

```bash
(cd web && npm install)
(cd web && npm run dev)
```

---

## Sessions

```bash
./alex sessions
./alex sessions pull <session-id>
./alex sessions cleanup --older-than 30d --keep-latest 20 --dry-run
```

---

## Tool safety (recommended)

Use `tool_preset` to control which tools the agent can call:

```bash
./alex config set tool_preset safe
./alex config set tool_preset read-only
./alex config set tool_preset full
```

---

## Validate

```bash
make fmt
make vet
make test
```

---

## Next steps

- Deployment: `docs/operations/DEPLOYMENT.md`
- Configuration reference: `docs/reference/CONFIG.md`
- Runtime/agent deep dive: `docs/AGENT.md`
