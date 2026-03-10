# Agents Teams E2E Testing

Updated: 2026-03-10

E2E testing for Agent Teams via Lark inject API (`POST :9090/api/dev/inject`). Tests run offline — no real Lark messages sent.

## Prerequisites

1. Claude CLI installed and authenticated
2. Server running: `alex dev restart backend` (debug on `:9090`)
3. `curl`, `jq`
4. Config includes templates: `claude_research`, `claude_analysis`, `claude_debate`

## Test Matrix

| Category | Cases | Validates |
|----------|-------|-----------|
| **A: Templates** | A1 `claude_research`, A2 `claude_analysis`, A3 `claude_debate` | Core template paths |
| **B: Edge Cases** | B1 minimal goal, B2 long goal, B3 code-syntax goal | Input handling |
| **C: Errors** | C1 nonexistent template, C2 missing goal, C3 template list | Error handling |
| **D: Context** | D1 analyst merge, D2 debate chain consumption | Context inheritance |
| **E: Override** | E1 per-role prompt override | Prompt customization |

## Running

```bash
./scripts/test_agents_teams_e2e.sh              # full suite
./scripts/test_agents_teams_e2e.sh --category C  # single category
./scripts/test_agents_teams_e2e.sh --case A1     # single case
./scripts/test_agents_teams_e2e.sh --dry-run     # list cases
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `INJECT_URL` | `http://127.0.0.1:9090/api/dev/inject` | Inject endpoint |
| `TIMEOUT_FAST` | `360` | Timeout for single-stage cases |
| `TIMEOUT_SLOW` | `720` | Timeout for multi-stage cases |
| `COOLDOWN` | `5` | Sleep between cases |

## Result Criteria

- **PASS**: completed as expected
- **PARTIAL**: output returned but keyword checks partial
- **FAIL**: API/dispatch failure or no usable output

Exit: `0` if no failures, `1` if any fail.

## Troubleshooting

```bash
# Claude CLI check
command -v claude && claude --version

# Server check
curl -s http://127.0.0.1:9090/api/dev/inject -X POST \
  -H "Content-Type: application/json" \
  -d '{"text":"ping","chat_id":"oc_test"}'

# Increase timeout for slow cases
TIMEOUT_SLOW=900 ./scripts/test_agents_teams_e2e.sh --case D2

# Inspect artifacts
jq . /tmp/agent_teams_e2e_<ts>/A2_resp.json
```
