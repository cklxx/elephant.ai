#!/usr/bin/env bash
#
# Start a local Chromium-based browser with a DevTools remote debugging port,
# then print a CDP endpoint you can set as `runtime.browser.cdp_url`.
#
# Notes:
# - To reuse your existing Chrome cookies/profile, QUIT the browser first so
#   the `--args` flags take effect.
# - The agent supports `cdp_url` as either:
#   - ws://... (webSocketDebuggerUrl), or
#   - http://127.0.0.1:<port> (DevTools HTTP endpoint; it will resolve to ws://).
#
set -euo pipefail

APP_NAME="Google Chrome"
PORT="9222"
WAIT_SECONDS="10"
USER_DATA_DIR=""
PROFILE_DIR=""

usage() {
  cat <<'EOF'
Usage:
  ./scripts/browser/start-cdp.sh [--app chrome|atlas|"<App Name>"] [--port 9222]
                                [--user-data-dir <dir>] [--profile-directory <name>]
                                [--wait-seconds 10]

Examples (macOS):
  ./scripts/browser/start-cdp.sh --app chrome --port 9222
  ./scripts/browser/start-cdp.sh --app "ChatGPT Atlas" --port 9223

Examples (Linux):
  BIN=google-chrome ./scripts/browser/start-cdp.sh --port 9222 --user-data-dir ~/.config/google-chrome

Output:
  Prints `http://127.0.0.1:<port>` to stdout (safe to paste into config as `cdp_url`).
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --app)
      APP_NAME="${2:-}"
      shift 2
      ;;
    --port)
      PORT="${2:-}"
      shift 2
      ;;
    --user-data-dir)
      USER_DATA_DIR="${2:-}"
      shift 2
      ;;
    --profile-directory)
      PROFILE_DIR="${2:-}"
      shift 2
      ;;
    --wait-seconds)
      WAIT_SECONDS="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

case "$(printf '%s' "$APP_NAME" | tr '[:upper:]' '[:lower:]')" in
  chrome)
    APP_NAME="Google Chrome"
    ;;
  atlas)
    APP_NAME="ChatGPT Atlas"
    ;;
esac

if [[ -n "${USER_DATA_DIR}" ]]; then
  USER_DATA_DIR="$(cd "${USER_DATA_DIR}" 2>/dev/null && pwd || echo "${USER_DATA_DIR}")"
fi

if command -v pgrep >/dev/null 2>&1; then
  if pgrep -x "$APP_NAME" >/dev/null 2>&1; then
    echo "Warning: ${APP_NAME} appears to be running. Quit it first so --args flags take effect." >&2
  fi
fi

OS="$(uname -s)"
if [[ "${OS}" == "Darwin" ]]; then
  cmd=(open -a "$APP_NAME" --args "--remote-debugging-port=${PORT}")
  if [[ -n "${USER_DATA_DIR}" ]]; then
    cmd+=("--user-data-dir=${USER_DATA_DIR}")
  fi
  if [[ -n "${PROFILE_DIR}" ]]; then
    cmd+=("--profile-directory=${PROFILE_DIR}")
  fi
  "${cmd[@]}" >/dev/null 2>&1 || "${cmd[@]}"
else
  BIN="${BIN:-}"
  if [[ -z "${BIN}" ]]; then
    if command -v google-chrome >/dev/null 2>&1; then
      BIN="google-chrome"
    elif command -v chromium >/dev/null 2>&1; then
      BIN="chromium"
    elif command -v chromium-browser >/dev/null 2>&1; then
      BIN="chromium-browser"
    else
      echo "Error: cannot find a Chrome/Chromium binary. Set BIN=... and retry." >&2
      exit 1
    fi
  fi
  cmd=("${BIN}" "--remote-debugging-port=${PORT}" "--no-first-run")
  if [[ -n "${USER_DATA_DIR}" ]]; then
    cmd+=("--user-data-dir=${USER_DATA_DIR}")
  fi
  if [[ -n "${PROFILE_DIR}" ]]; then
    cmd+=("--profile-directory=${PROFILE_DIR}")
  fi
  "${cmd[@]}" >/dev/null 2>&1 &
fi

BASE_URL="http://127.0.0.1:${PORT}"
VERSION_URL="${BASE_URL}/json/version"
echo "DevTools endpoint: ${BASE_URL}" >&2

deadline=$((SECONDS + WAIT_SECONDS))
while (( SECONDS < deadline )); do
  if command -v curl >/dev/null 2>&1 && curl -fsS "${VERSION_URL}" >/dev/null 2>&1; then
    break
  fi
  sleep 0.2
done

if command -v curl >/dev/null 2>&1 && curl -fsS "${VERSION_URL}" >/dev/null 2>&1; then
  if command -v python3 >/dev/null 2>&1; then
    ws_url="$(curl -fsS "${VERSION_URL}" | python3 -c 'import json,sys; print(json.load(sys.stdin).get("webSocketDebuggerUrl",""))' || true)"
    if [[ -n "${ws_url}" ]]; then
      echo "webSocketDebuggerUrl: ${ws_url}" >&2
    fi
  fi
else
  echo "Warning: DevTools endpoint not reachable at ${VERSION_URL} (still printing base url)." >&2
fi

printf '%s\n' "${BASE_URL}"

