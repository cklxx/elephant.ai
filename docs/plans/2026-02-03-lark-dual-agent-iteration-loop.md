# Lark Dual-Agent Iteration Loop

**Date:** 2026-02-03
**Status:** Draft
**Author:** cklxx

---

## Goal

用户只在 Lark 上交互，整个 coding → test → fix → restart 闭环全自动。
Same folder, two scripts, two Lark bots, continuous iteration.

---

## Architecture

```
~/code/elephant.ai (single folder)
├── lark.sh    ← Primary (coding bot, port 8080, ~/.alex/config.yaml)
├── lark2.sh   ← Secondary (test bot, port 8081, ~/.alex/config-secondary.yaml)
└── ~/.alex/   (shared)

Continuous Loop (all status visible on Lark):
  User → Bot A "implement X"
    → primary agent codes → commit → push → ./lark2.sh restart
    → secondary: build → test
    → pass:  ./lark.sh restart → Bot B notify "tests passed, primary restarted"
             → primary back online, waiting for next Lark message
    → fail:  Bot B notify "tests failed, auto-fixing..."
             → codex fix → rebuild → retest (loop)
             → eventually pass → ./lark.sh restart → notify
```

Two processes = two `ALEX_CONFIG_PATH` + two `PORT`. Same binary, same source.
User sees all status on Lark. Never leaves Lark.

---

## Key Design Points

1. **Agent instructions in CLAUDE.md / AGENTS.md**: Coding agent 自然读到 "完成后 run `./lark2.sh restart`"
2. **Fail path 自动修复**: lark2.sh 直接调 codex/claude 修，自动 loop 直到修好
3. **全程进展推送 Lark**: build → test → fix → restart 每一步状态发到用户 Lark 群
4. **Success = continue**: Restart primary 后立刻在线等下条消息
5. **All on Lark**: 用户只发任务、收结果。中间部署进展自动推送

---

## Config

### Primary: `~/.alex/config.yaml` — existing, no changes

### Secondary: `~/.alex/config-secondary.yaml` — new

Only differences: Lark bot credentials + session_prefix.

```yaml
# ~/.alex/config-secondary.yaml
runtime:
  api_key: ${OPENAI_API_KEY}
  ark_api_key: ${ARK_API_KEY}
  base_url: https://ark-cn-beijing.bytedance.net/api/v3
  llm_model: ep-20251218160436-v9tkk
  llm_provider: auto
  llm_small_model: ep-20251225161921-vlzzn
  max_tokens: 12800
  tavily_api_key: ${TAVILY_API_KEY}
  verbose: false
  proactive:
    enabled: false
    memory:
      enabled: true

channels:
  lark:
    enabled: true
    app_id: "${SECONDARY_LARK_APP_ID}"
    app_secret: "${SECONDARY_LARK_APP_SECRET}"
    base_domain: https://open.larkoffice.com
    workspace_dir: /Users/bytedance/code/elephant.ai
    session_prefix: secondary
    allow_direct: true
    allow_groups: true
    memory_enabled: true
    reply_timeout_seconds: 18000

auth:
  database_url: postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable
  jwt_secret: ${AUTH_JWT_SECRET}
  bootstrap_email: admin@example.com
  bootstrap_password: ${AUTH_BOOTSTRAP_PASSWORD}

session:
  database_url: postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable
```

---

## Scripts

### `lark.sh` — Primary

```bash
#!/usr/bin/env bash
# lark.sh — Primary Lark agent (coding bot)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
source "$ROOT/scripts/lib/common/logging.sh"
source "$ROOT/scripts/lib/common/process.sh"

PORT="${PRIMARY_PORT:-8080}"
PID="$ROOT/.pids/lark.pid"
LOG="$ROOT/logs/lark.log"
BIN="$ROOT/.build/alex-server"
CONFIG="$HOME/.alex/config.yaml"
MAX_RETRIES=3

mkdir -p "$ROOT/.pids" "$ROOT/logs"

build() {
  log_info "Building..."
  CGO_ENABLED=0 go build -o "$BIN" "$ROOT/cmd/server/"
}

ensure_db() {
  pg_isready -h localhost -p 5432 -q 2>/dev/null || "$ROOT/scripts/setup_local_auth_db.sh"
}

stop() {
  [[ -f "$PID" ]] || return 0
  local p; p=$(cat "$PID")
  kill -0 "$p" 2>/dev/null && kill "$p" && log_info "Stopped primary (pid=$p)"
  rm -f "$PID"
}

start() {
  ensure_db
  build || { auto_fix "Build failed: $(tail -30 "$LOG")"; return; }
  stop 2>/dev/null || true

  ALEX_CONFIG_PATH="$CONFIG" PORT="$PORT" ALEX_SERVER_PORT="$PORT" \
    nohup "$BIN" >> "$LOG" 2>&1 &
  echo $! > "$PID"

  local i=0
  while ! curl -sf "http://localhost:${PORT}/health" >/dev/null 2>&1; do
    i=$((i + 1))
    [[ $i -ge 30 ]] && { auto_fix "Startup failed: $(tail -50 "$LOG")"; return; }
    sleep 1
  done
  log_success "Primary healthy on :$PORT"
}

auto_fix() {
  local err="$1"
  for attempt in $(seq 1 $MAX_RETRIES); do
    log_warn "Auto-fix attempt $attempt/$MAX_RETRIES..."
    codex --approval-policy auto-edit --quiet \
      "Server failed to start. Fix only this error: $err" 2>&1 | tee -a "$LOG" || true

    cd "$ROOT"
    git diff --quiet && git diff --cached --quiet && continue
    git add -A
    git commit -m "fix: auto-fix primary (attempt $attempt)" || true
    git push || true

    build || continue
    stop 2>/dev/null || true
    ALEX_CONFIG_PATH="$CONFIG" PORT="$PORT" ALEX_SERVER_PORT="$PORT" \
      nohup "$BIN" >> "$LOG" 2>&1 &
    echo $! > "$PID"
    sleep 3
    curl -sf "http://localhost:${PORT}/health" >/dev/null 2>&1 && {
      log_success "Auto-fix succeeded"; return 0
    }
    err="$(tail -50 "$LOG")"
  done
  log_error "Auto-fix exhausted. Manual fix needed."
  return 1
}

case "${1:-start}" in
  start|up)   start ;;
  stop|down)  stop ;;
  restart)    stop; start ;;
  build)      build ;;
  logs)       tail -f "$LOG" ;;
  *)          echo "Usage: $0 {start|stop|restart|build|logs}" ;;
esac
```

### `lark2.sh` — Secondary

```bash
#!/usr/bin/env bash
# lark2.sh — Secondary Lark agent (test + restart + notify)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
source "$ROOT/scripts/lib/common/logging.sh"
source "$ROOT/scripts/lib/common/process.sh"

PORT="${SECONDARY_PORT:-8081}"
PID="$ROOT/.pids/lark2.pid"
LOG="$ROOT/logs/lark2.log"
BIN="$ROOT/.build/alex-server"
CONFIG="$HOME/.alex/config-secondary.yaml"
PRIMARY_CHAT="${PRIMARY_LARK_CHAT_ID:-}"
MAX_RETRIES=3
MAX_CYCLES=5

mkdir -p "$ROOT/.pids" "$ROOT/logs"

stop() {
  [[ -f "$PID" ]] || return 0
  local p; p=$(cat "$PID")
  kill -0 "$p" 2>/dev/null && kill "$p" && log_info "Stopped secondary (pid=$p)"
  rm -f "$PID"
}

start_server() {
  stop 2>/dev/null || true
  ALEX_CONFIG_PATH="$CONFIG" PORT="$PORT" ALEX_SERVER_PORT="$PORT" \
    nohup "$BIN" >> "$LOG" 2>&1 &
  echo $! > "$PID"

  local i=0
  while ! curl -sf "http://localhost:${PORT}/health" >/dev/null 2>&1; do
    i=$((i + 1))
    [[ $i -ge 30 ]] && return 1
    sleep 1
  done
  log_success "Secondary healthy on :$PORT"
}

test_all() {
  log_info "Running tests..."
  cd "$ROOT"
  golangci-lint run ./... >> "$LOG" 2>&1 || log_warn "Lint warnings"
  CGO_ENABLED=0 go test -count=1 ./... 2>&1 | tee "$ROOT/logs/lark2-test.log"
}

# Progress notification → Lark (via Bot B's server)
notify() {
  [[ -z "$PRIMARY_CHAT" ]] && return
  curl -sf -X POST "http://localhost:${PORT}/api/lark/send" \
    -H "Content-Type: application/json" \
    -d "{\"chat_id\":\"${PRIMARY_CHAT}\",\"text\":\"$1\"}" || true
}

# Direct codex invocation for auto-fix
fix_with_codex() {
  local err="$1"
  for attempt in $(seq 1 $MAX_RETRIES); do
    log_warn "Codex fix attempt $attempt/$MAX_RETRIES..."
    codex --approval-policy auto-edit --quiet \
      "Build/test failed. Fix only this error: $err" 2>&1 | tee -a "$LOG" || true

    cd "$ROOT"
    git diff --quiet && git diff --cached --quiet && continue
    git add -A
    git commit -m "fix: auto-fix from test (attempt $attempt)" || true
    git push || true
    return 0  # made changes, caller should retry
  done
  return 1  # codex couldn't fix
}

run() {
  local cycle=0

  while [[ $cycle -lt $MAX_CYCLES ]]; do
    cycle=$((cycle + 1))
    log_info "=== Cycle $cycle/$MAX_CYCLES ==="

    # Build
    notify "[cycle $cycle/$MAX_CYCLES] Building..."
    if ! CGO_ENABLED=0 go build -o "$BIN" "$ROOT/cmd/server/" 2>&1 | tee -a "$LOG"; then
      notify "[cycle $cycle] Build failed, auto-fixing..."
      fix_with_codex "Build failed: $(tail -30 "$LOG")" || break
      continue
    fi

    # Start secondary server
    notify "[cycle $cycle] Starting test server..."
    if ! start_server; then
      notify "[cycle $cycle] Server start failed, auto-fixing..."
      fix_with_codex "$(tail -50 "$LOG")" || break
      continue
    fi

    # Test
    notify "[cycle $cycle] Running tests..."
    if test_all; then
      notify "[cycle $cycle] Tests passed. Restarting primary..."
      "$ROOT/lark.sh" restart
      notify "Primary restarted. Ready."
      return 0
    else
      local err; err="$(tail -30 "$ROOT/logs/lark2-test.log")"
      notify "[cycle $cycle] Tests failed, auto-fixing..."
      fix_with_codex "$err" || break
    fi
  done

  notify "Auto-fix exhausted ($MAX_CYCLES cycles). Manual fix needed."
  return 1
}

case "${1:-start}" in
  start|up)   run ;;
  stop|down)  stop ;;
  restart)    stop; run ;;
  test)       test_all ;;
  logs)       tail -f "$LOG" ;;
  *)          echo "Usage: $0 {start|stop|restart|test|logs}" ;;
esac
```

---

## Agent Instructions

写在项目根目录 `CLAUDE.md` 和 `AGENTS.md` 里，coding agent (Claude Code / Codex) 启动时自动读取：

```markdown
## Deployment workflow

Every time you finish code changes:
1. Run lint and tests: golangci-lint run ./... && go test ./...
2. git add + commit + push
3. Run: ./lark2.sh restart
Never skip step 3. This triggers automated validation and deployment.
```

不需要改 Go 代码、不需要改 agent preset — 文件约定即可。

---

## Complete Flow (Lark Closed Loop)

```
User (Lark)              lark.sh (Bot A)         lark2.sh (Bot B → notify)
    │                          │                        │
    │── "implement X" ────────►│                        │
    │◄── Bot A: "working..." ──│                        │
    │                          │ [code]                 │
    │◄── Bot A: "done, code…" ─│                        │
    │                          │ [commit + push]        │
    │                          │ [./lark2.sh restart]──►│
    │                          │                        │
    │◄── Bot B: "Building..." ─┼────────────────────────│
    │◄── Bot B: "Testing..." ──┼────────────────────────│
    │                          │                        │
    │                          │                  pass? │
    │◄── Bot B: "Tests passed. ┼────────────────────────│
    │    Restarting primary…"  │◄── ./lark.sh restart ──│
    │◄── Bot B: "Primary      ┼────────────────────────│
    │    restarted. Ready." ───│                        │
    │                          │                        │
    │── "next task..." ───────►│                        │
    │                          │                        │
    │                          │                  fail? │
    │◄── Bot B: "Tests failed, ┼────────────────────────│
    │    auto-fixing..." ──────│            [codex fix] │
    │                          │            [rebuild]   │
    │◄── Bot B: "Building..." ─┼────────────────────────│
    │◄── Bot B: "Testing..." ──┼────────────────────────│
    │                          │               ...loop  │
    │◄── Bot B: "Primary      ┼────────────────────────│
    │    restarted. Ready." ───│                        │
```

User 全程在 Lark 看到每一步进展。

---

## New Endpoint: `POST /api/lark/send`

```
POST /api/lark/send  {"chat_id":"oc_xxx","text":"message"}
```

Bot B uses this to send status to user's chat.

---

## Files

| File | Action |
|------|--------|
| `~/.alex/config-secondary.yaml` | Create |
| `lark.sh` | Create |
| `lark2.sh` | Create |
| `internal/server/http/api_handler_lark_send.go` | Create |
| `internal/server/http/router.go` | Modify |

## Env Vars

```bash
SECONDARY_LARK_APP_ID=cli_xxx
SECONDARY_LARK_APP_SECRET=xxx
PRIMARY_LARK_CHAT_ID=oc_xxx          # user's chat where Bot B sends status
SECONDARY_PORT=8081
```

## Safeguards

- `MAX_CYCLES=5` in lark2.sh prevents infinite test→fix loop
- `MAX_RETRIES=3` per codex fix round
- Both bots must be in the same Lark group (or Bot B needs DM access)
- Only lark.sh runs `ensure_db` (no migration race)

## Progress

- [ ] Create `~/.alex/config-secondary.yaml`
- [ ] Create `lark.sh`
- [ ] Create `lark2.sh`
- [ ] Implement `POST /api/lark/send`
- [ ] Add agent system prompt instruction for `./lark2.sh restart`
- [ ] Create second Lark bot (Bot B)
- [ ] Add Bot B to user's chat group
- [ ] Test full loop
