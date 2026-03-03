# Agents Teams E2E Testing Guide

Updated: 2026-03-03

## Overview

Systematic end-to-end testing for the Agents Teams feature via Lark inject API (`POST :9090/api/dev/inject`).
Tests run in offline mode: no real Lark outbound message is sent.

Current default suite is **claude-only local profile** and is implemented by:
- `scripts/test_agents_teams_e2e.sh`

The suite currently covers **12 cases across 5 categories**.

## Prerequisites

1. **Claude CLI** installed and authenticated
2. **Server** running: `alex dev restart backend` (debug server on `:9090`)
3. **Dependencies**: `curl`, `jq`
4. **Config** (`~/.alex/config.yaml`) includes templates used by the script:
   - `claude_research`
   - `claude_analysis`
   - `claude_debate`

## Test Matrix (Claude-only)

### Category A: Core Templates

| Case | Template | Validates |
|------|----------|-----------|
| A1 | `claude_research` | Single-stage research path |
| A2 | `claude_analysis` | Parallel dual-perspective + synthesis |
| A3 | `claude_debate` | Debate mode chain (analyst + challenger + reviewer) |

### Category B: Input Edge Cases

| Case | Template | Validates |
|------|----------|-----------|
| B1 | `claude_research` | Minimal single-word goal |
| B2 | `claude_research` | Long multi-constraint goal |
| B3 | `claude_research` | Goal containing code syntax |

### Category C: Error Handling

| Case | Action | Validates |
|------|--------|-----------|
| C1 | `template=nonexistent_template` | Graceful not-found handling |
| C2 | Missing `goal` | Parameter validation |
| C3 | `template=list` | Template listing behavior |

### Category D: Context Inheritance

| Case | Template | Validates |
|------|----------|-----------|
| D1 | `claude_analysis` | Synthesizer merges both analyst perspectives |
| D2 | `claude_debate` | Reviewer can consume analyst + challenger outputs |

### Category E: Prompt Override

| Case | Template | Validates |
|------|----------|-----------|
| E1 | `claude_research` | Per-role prompt override via `prompts` |

## Running Tests

```bash
# Full suite (default order: C -> A -> B -> D -> E)
./scripts/test_agents_teams_e2e.sh

# Single category
./scripts/test_agents_teams_e2e.sh --category C

# Single case
./scripts/test_agents_teams_e2e.sh --case A1

# Dry run (list cases)
./scripts/test_agents_teams_e2e.sh --dry-run

# Custom inject URL
./scripts/test_agents_teams_e2e.sh --url http://localhost:9090/api/dev/inject
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INJECT_URL` | `http://127.0.0.1:9090/api/dev/inject` | Inject API endpoint |
| `SENDER_ID` | `ou_e2e_claude_teams` | Sender id prefix |
| `TIMEOUT_FAST` | `360` | Timeout (seconds) for short/single-stage cases |
| `TIMEOUT_SLOW` | `720` | Timeout (seconds) for multi-stage/debate cases |
| `COOLDOWN` | `5` | Sleep seconds between cases |

## Result Criteria

| Status | Meaning |
|--------|---------|
| **PASS** | Case completed as expected |
| **PARTIAL** | Case returned output but signal/keyword checks were partial |
| **FAIL** | API/dispatch failure or no usable output |

Exit behavior:
- `0`: all pass OR partial-without-fail
- `1`: at least one fail

## Troubleshooting

### Claude CLI not available or unauthenticated

```bash
command -v claude
claude --version
```

If not available/authenticated, install/login Claude CLI before running the suite.

### Timeout on slow cases

```bash
TIMEOUT_SLOW=900 ./scripts/test_agents_teams_e2e.sh --case D2
```

### Server not reachable

```bash
curl -s http://127.0.0.1:9090/api/dev/inject -X POST \
  -H "Content-Type: application/json" \
  -d '{"text":"ping","chat_id":"oc_test"}'

alex dev restart backend
```

### Inspect response artifacts

```bash
# Find latest run dir
ls -td /tmp/agent_teams_e2e_* | head -1

# Inspect one case
jq . /tmp/agent_teams_e2e_<ts>/A2_resp.json
```

## Historical Note

The earlier kimi-oriented matrix and result log were used in prior phases and are now **legacy snapshots**.
Current maintained baseline for this guide is the claude-only script above.
