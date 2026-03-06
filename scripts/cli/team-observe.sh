#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LINES="${TEAM_OBSERVE_LINES:-80}"
RUNTIME_ROOT="${TEAM_RUNTIME_ROOT:-}"
WATCH_INTERVAL=""
ACTIVE_ONLY=0
ACTIVE_WINDOW_SEC="${TEAM_OBSERVE_ACTIVE_WINDOW_SEC:-1800}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --lines)
      LINES="$2"; shift 2 ;;
    --runtime-root)
      RUNTIME_ROOT="$2"; shift 2 ;;
    --watch)
      WATCH_INTERVAL="${2:-5}"; shift 2 ;;
    --active-only)
      ACTIVE_ONLY=1; shift ;;
    -h|--help)
      cat <<'EOF'
usage: bash scripts/cli/team-observe.sh [--lines N] [--runtime-root PATH] [--watch SEC] [--active-only]

Shows:
  1) codex processes
  2) team runtime summary
  3) latest team terminal snapshot (if available)

Options:
  --watch SEC     auto refresh every N seconds
  --active-only   show only active codex / recently-updated teams

Env:
  TEAM_OBSERVE_LINES=200
  TEAM_RUNTIME_ROOT=.elephant/tasks/_team_runtime
  TEAM_OBSERVE_ACTIVE_WINDOW_SEC=1800
EOF
      exit 0 ;;
    *)
      echo "unknown arg: $1" >&2
      exit 2 ;;
  esac
done

run_alex() {
  (cd "$ROOT" && go run ./cmd/alex team "$@")
}

status_args=(status --all --tail 20 --json)
if [[ -n "$RUNTIME_ROOT" ]]; then
  status_args+=(--runtime-root "$RUNTIME_ROOT")
fi

render_once() {
  local status_json latest_team

  echo "== Codex processes =="
  if [[ "$ACTIVE_ONLY" == "1" ]]; then
    ps -Ao pid,etime,%cpu,%mem,command \
      | awk '$0 ~ /\/codex\/codex --dangerously-bypass-approvals-and-sandbox/ && $3+0 >= 1 {print}' || true
  else
    ps -Ao pid,etime,%cpu,%mem,command \
      | awk '$0 ~ /\/codex\/codex --dangerously-bypass-approvals-and-sandbox/ {print}' || true
  fi

  echo
  echo "== Team runtime summary =="
  status_json="$(run_alex "${status_args[@]}" 2>/dev/null || true)"
  if [[ -z "$status_json" ]]; then
    echo "no team runtime found"
    return 0
  fi

  python3 - <<'PY' "$status_json" "$ACTIVE_ONLY" "$ACTIVE_WINDOW_SEC"
import json, sys
from datetime import datetime, timezone

report = json.loads(sys.argv[1])
active_only = sys.argv[2] == '1'
active_window_sec = int(sys.argv[3])
entries = report.get('entries', [])
now = datetime.now(timezone.utc)

def parse_ts(raw):
    if not raw:
        return None
    raw = raw.replace('Z', '+00:00')
    try:
        return datetime.fromisoformat(raw)
    except Exception:
        return None

filtered = []
for e in entries:
    recent_events = e.get('recent_events') or []
    latest = None
    for ev in recent_events:
        ts = parse_ts(ev.get('timestamp'))
        if ts and (latest is None or ts > latest):
            latest = ts
    is_active = latest is not None and (now - latest).total_seconds() <= active_window_sec
    if active_only and not is_active:
        continue
    filtered.append((e, latest, is_active))

print(f"entries={len(filtered)}")
for i, (e, latest, is_active) in enumerate(filtered[:8], 1):
    roles = e.get('roles') or []
    role = roles[0].get('RoleID') if roles else '-'
    pane = roles[0].get('TmuxPane') if roles else '-'
    age = '-' if latest is None else f"{int((now-latest).total_seconds()/60)}m"
    flag = 'active' if is_active else 'idle'
    print(f"[{i}] {flag:6} age={age:>4} team={e.get('team_id')} template={e.get('template')} role={role} pane={pane}")
PY

  latest_team="$(python3 - <<'PY' "$status_json" "$ACTIVE_ONLY" "$ACTIVE_WINDOW_SEC"
import json, sys
from datetime import datetime, timezone
report = json.loads(sys.argv[1])
active_only = sys.argv[2] == '1'
active_window_sec = int(sys.argv[3])
entries = report.get('entries', [])
now = datetime.now(timezone.utc)

def parse_ts(raw):
    if not raw:
        return None
    raw = raw.replace('Z', '+00:00')
    try:
        return datetime.fromisoformat(raw)
    except Exception:
        return None

for e in entries:
    recent_events = e.get('recent_events') or []
    latest = None
    for ev in recent_events:
        ts = parse_ts(ev.get('timestamp'))
        if ts and (latest is None or ts > latest):
            latest = ts
    is_active = latest is not None and (now - latest).total_seconds() <= active_window_sec
    if active_only and not is_active:
        continue
    roles = e.get('roles') or []
    if roles and roles[0].get('TmuxPane'):
        print(e.get('team_id') or '')
        break
PY
)"

  echo
  echo "== Latest team terminal snapshot =="
  if [[ -z "$latest_team" ]]; then
    echo "no matching team with visible pane"
    return 0
  fi

  term_args=(terminal --team-id "$latest_team" --mode capture --lines "$LINES")
  if [[ -n "$RUNTIME_ROOT" ]]; then
    term_args+=(--runtime-root "$RUNTIME_ROOT")
  fi
  run_alex "${term_args[@]}" || true
}

if [[ -n "$WATCH_INTERVAL" ]]; then
  while true; do
    clear || true
    date '+%Y-%m-%d %H:%M:%S'
    echo "watch=${WATCH_INTERVAL}s active_only=${ACTIVE_ONLY} lines=${LINES}"
    echo
    render_once
    sleep "$WATCH_INTERVAL"
  done
else
  render_once
fi

