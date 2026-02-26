# Agents Teams E2E Testing Guide

## Overview

Systematic end-to-end testing for the Agents Teams feature using the Lark inject API (`POST :9090/api/dev/inject`). Tests run in offline mode — no real Lark messages are sent.

The suite covers 14 test cases across 5 categories, validating team orchestration from simple 2-stage pipelines to complex parallel fan-in patterns with mixed agent types (kimi + internal).

## Prerequisites

1. **kimi CLI** installed and accessible in `$PATH`
2. **Config** — `~/.alex/config.yaml` must contain 3 team templates:
   - `kimi_research` — 2 roles, 2 stages (parallel research → synthesis)
   - `technical_analysis` — 3 roles, 3 stages (serial pipeline)
   - `competitive_review` — 4 roles, 2 stages (3-way parallel → judge)
3. **Server** running: `alex dev restart backend` (debug server on `:9090`)
4. **Dependencies**: `curl`, `jq`

## Test Matrix

### Category A: Core Functionality

| Case | Template | Goal | Validates |
|------|----------|------|-----------|
| A1 | kimi_research | Go error wrapping with `fmt.Errorf %w` | Baseline 2-stage end-to-end |
| A2 | technical_analysis | Redis vs Memcached for session caching | 3-stage serial + context chain |
| A3 | competitive_review | Rust vs Go for backend microservices | 3-way parallel fan-out → fan-in |

### Category B: Input Boundaries

| Case | Goal Characteristic | Validates |
|------|-------------------|-----------|
| B1 | Single word: "Kubernetes" | Minimal input doesn't crash |
| B2 | 3 comparisons, long sentence | Long goal propagation |
| B3 | `select {}` special characters | Safe character escaping |
| B4 | Pure English instruction | Language routing correctness |

### Category C: Error Handling & Degradation

| Case | Action | Validates |
|------|--------|-----------|
| C1 | template=nonexistent | Graceful error for unknown template |
| C2 | Missing goal parameter | Parameter validation |
| C3 | template=list | Template metadata listing |
| C4 | Same chat_id twice | Conflict/duplicate handling |

### Category D: Prompt Override

| Case | Action | Validates |
|------|--------|-----------|
| D1 | Override researcher prompt | Selective role prompt replacement |

### Category E: Complex Real-World Scenarios

| Case | Template | Goal | Validates |
|------|----------|------|-----------|
| E1 | competitive_review | PostgreSQL vs MySQL vs CockroachDB multi-region | Real tech selection quality |
| E2 | technical_analysis | Event sourcing vs CRUD for financial transactions | Real architecture analysis quality |

## Running Tests

```bash
# Full suite (recommended order: C → B → A → D → E)
./scripts/test_agents_teams_e2e.sh

# Single category
./scripts/test_agents_teams_e2e.sh --category C

# Single case
./scripts/test_agents_teams_e2e.sh --case A1

# Dry run — show all cases without executing
./scripts/test_agents_teams_e2e.sh --dry-run

# Custom server URL
./scripts/test_agents_teams_e2e.sh --url http://localhost:9090/api/dev/inject
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INJECT_URL` | `http://127.0.0.1:9090/api/dev/inject` | Inject API endpoint |
| `TIMEOUT_KIMI` | `300` | Timeout (seconds) for kimi-heavy cases |
| `TIMEOUT_INTERNAL` | `120` | Timeout (seconds) for internal-only cases |
| `COOLDOWN_KIMI` | `30` | Cooldown between kimi cases |
| `COOLDOWN_HEAVY` | `60` | Cooldown between complex cases (Category E) |

## Result Criteria

| Status | Meaning |
|--------|---------|
| **PASS** | All roles completed, expected replies received, 0 errors |
| **PARTIAL** | Kimi roles completed but hit rate limits or internal roles degraded — not a code bug |
| **FAIL** | Dispatch error, crash, code-level failure, or 0 replies |

### Final Suite Verdict

- **ALL PASSED** (exit 0): Every case is PASS
- **SUITE PARTIAL** (exit 0): Some cases PARTIAL, none FAIL
- **SUITE FAILED** (exit 1): One or more cases FAIL

### Empty Assistant Check

The suite tracks `empty assistant` errors across all responses. Expected count after full run: **0**. Any non-zero value indicates the empty-assistant-message fix has regressed.

## Troubleshooting

### 429 Rate Limit from kimi

Kimi has aggressive rate limiting. The script uses cooldowns between cases (30s default, 60s for heavy). If you still hit 429s:

```bash
# Increase cooldown
COOLDOWN_KIMI=60 COOLDOWN_HEAVY=120 ./scripts/test_agents_teams_e2e.sh
```

Or run categories individually with manual pauses between them.

### Timeout

Default timeout is 5 minutes for kimi cases. Complex templates (3 stages, 3 parallel) may need more:

```bash
TIMEOUT_KIMI=600 ./scripts/test_agents_teams_e2e.sh --case E1
```

### Empty Assistant Errors

If `empty assistant errors > 0`, check:
1. The `fix(llm): drop empty assistant messages` commit is deployed
2. Look at the response file: `jq . /tmp/agents_teams_e2e_*/CASE_resp.json`
3. Verify kimi bridge is returning well-formed JSONL

### Server Not Reachable

```bash
# Check server is running
curl -s http://127.0.0.1:9090/api/dev/inject -X POST \
  -H "Content-Type: application/json" \
  -d '{"text":"ping","chat_id":"oc_test"}'

# Restart if needed
alex dev restart backend
```

### Inspecting Responses

All request/response files are saved to a timestamped temp directory:

```bash
# Find latest run
ls -td /tmp/agents_teams_e2e_* | head -1

# Inspect a specific case
jq . /tmp/agents_teams_e2e_<ts>/A2_resp.json

# See all replies for a case
jq '.replies[] | {method, msg_type, content: .content[:200]}' /tmp/agents_teams_e2e_<ts>/A2_resp.json
```

## Test Results Log

Record results here after each execution run.

| Date | Categories | Pass | Partial | Fail | Empty Asst | Notes |
|------|-----------|------|---------|------|------------|-------|
| _yyyy-mm-dd_ | _A-E_ | _n_ | _n_ | _n_ | _n_ | _notes_ |
