# elephant.ai Quickstart
> Last updated: 2025-12-14

This guide gets you from `git clone` to running the `alex` CLI/TUI, server, and web UI. Configure an OpenAI-compatible LLM provider via the shared runtime config system (`docs/reference/CONFIG.md`).

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

## Configure

```bash
cp examples/config/core-config-example.json ~/.alex-config.json
export OPENAI_API_KEY="sk-..."
export LLM_PROVIDER="openai"
export LLM_BASE_URL="https://api.openai.com/v1"
export LLM_MODEL="gpt-4o-mini"
```

Optional managed overrides (persistent):

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

Use `tool_preset` to control which tools the CLI agent can call (web mode ignores it and enables all non-local tools):

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
