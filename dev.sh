#!/usr/bin/env bash
###############################################################################
# elephant.ai - Local Development Helper (thin dispatcher)
#
# This script delegates to `alex dev` for all process management.
# Build alex first: go build -o ./alex ./cmd/alex
#
# Usage:
#   ./dev.sh                    # Start all (backend + web + lark)
#   ./dev.sh lark [cmd]         # Lark stack (default cmd=up)
#   ./dev.sh up|start           # Start backend + web only
#   ./dev.sh up --lark          # Start backend + web + lark
#   ./dev.sh down|stop          # Stop backend + web
#   ./dev.sh status             # Show status + ports
#   ./dev.sh logs [server|web]  # Tail logs
#   ./dev.sh logs-ui            # Start services and open diagnostics workbench
#   ./dev.sh test               # Go tests (CI parity)
#   ./dev.sh lint               # Go + web lint
#
# Env:
#   SERVER_PORT=8080            # Backend port override (default 8080)
#   WEB_PORT=3000               # Web port override (default 3000)
#   AUTO_STOP_CONFLICTING_PORTS=1 # Auto-stop our backend/web conflicts (default 1)
#   AUTO_HEAL_WEB_NEXT=1        # Auto-heal Next.js .next/dev ENOENT corruption (default 1)
#   AUTH_JWT_SECRET=...         # Auth secret (default: dev-secret-change-me)
#   ALEX_CGO_MODE=auto|on|off    # Auto-select CGO for builds (default auto)
###############################################################################

set -euo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Source dotenv and logging for bootstrap
source "${SCRIPT_DIR}/scripts/lib/common/logging.sh"
source "${SCRIPT_DIR}/scripts/lib/common/dotenv.sh"
load_dotenv_file "${SCRIPT_DIR}/.env"

export AUTH_JWT_SECRET="${AUTH_JWT_SECRET:-dev-secret-change-me}"

# ---------------------------------------------------------------------------
# Ensure alex binary is available
# ---------------------------------------------------------------------------

ALEX_BIN="${SCRIPT_DIR}/alex"

ensure_alex_binary() {
  if [[ -x "${ALEX_BIN}" ]]; then
    return 0
  fi
  log_info "Building alex CLI..."
  (cd "${SCRIPT_DIR}" && CGO_ENABLED=0 go build -o "${ALEX_BIN}" ./cmd/alex) || die "Failed to build alex CLI"
  log_success "Built ${ALEX_BIN}"
}

# ---------------------------------------------------------------------------
# setup-cgo (standalone, not in alex dev)
# ---------------------------------------------------------------------------

cmd_setup_cgo() {
  "${SCRIPT_DIR}/scripts/setup_cgo_sqlite.sh"
}

# ---------------------------------------------------------------------------
# Command dispatch
# ---------------------------------------------------------------------------

usage() {
  cat <<EOF
elephant.ai dev helper

Usage:
  ./dev.sh                 # Start backend + web + lark (default)
  ./dev.sh [command]

Commands:
  all-up         Start backend + web + lark (same as no args)
  lark [cmd]     Manage lark stack (default: up)
                 cmd: up|down|restart|status|logs|doctor|cycle
  up|start       Start backend + web only (background)
  up --lark      Start backend + web + lark
  down|stop      Stop backend + web
  down-all       Stop everything and reset bootstrap
  status         Show status + ports
  logs           Tail logs (optional: server|web)
  logs-ui        Start services and open the diagnostics workbench
  test           Run Go tests (CI parity)
  lint           Run Go + web lint
  setup-cgo      Install CGO sqlite dependencies

Legacy aliases still accepted:
  all-down | all-status | lark-up | lark-down | lark-status | lark-logs
EOF
}

cmd="${1:-all-up}"
shift || true

# setup-cgo doesn't need the alex binary
if [[ "${cmd}" == "setup-cgo" ]]; then
  cmd_setup_cgo
  exit 0
fi

if [[ "${cmd}" == "help" || "${cmd}" == "-h" || "${cmd}" == "--help" ]]; then
  usage
  exit 0
fi

ensure_alex_binary

case "$cmd" in
  # Core commands — delegate directly
  up|start)        exec "${ALEX_BIN}" dev up "$@" ;;
  down|stop)       exec "${ALEX_BIN}" dev down "$@" ;;
  status)          exec "${ALEX_BIN}" dev status "$@" ;;
  logs)            exec "${ALEX_BIN}" dev logs "$@" ;;
  restart)         exec "${ALEX_BIN}" dev restart "$@" ;;
  test)            exec "${ALEX_BIN}" dev test "$@" ;;
  lint)            exec "${ALEX_BIN}" dev lint "$@" ;;
  logs-ui|log-ui|analyze-logs) exec "${ALEX_BIN}" dev logs-ui "$@" ;;

  # Lark commands
  lark)            exec "${ALEX_BIN}" dev lark "$@" ;;

  # Composite commands
  all-up)          exec "${ALEX_BIN}" dev up --lark "$@" ;;
  all-down)
    "${ALEX_BIN}" dev down --all "$@"
    ;;
  all-status)      exec "${ALEX_BIN}" dev status "$@" ;;
  down-all|stop-all)
    "${ALEX_BIN}" dev down --all "$@"
    ;;

  # Legacy lark aliases
  lark-up)
    log_warn "Deprecated: './dev.sh lark-up'. Use './dev.sh lark up' instead."
    exec "${ALEX_BIN}" dev lark start "$@"
    ;;
  lark-down)
    log_warn "Deprecated: './dev.sh lark-down'. Use './dev.sh lark down' instead."
    exec "${ALEX_BIN}" dev lark stop "$@"
    ;;
  lark-status)
    log_warn "Deprecated: './dev.sh lark-status'. Use './dev.sh lark status' instead."
    exec "${ALEX_BIN}" dev lark status "$@"
    ;;
  lark-logs)
    log_warn "Deprecated: './dev.sh lark-logs'. Use './dev.sh lark logs' instead."
    exec "${ALEX_BIN}" dev lark logs "$@"
    ;;

  *)
    die "Unknown command: ${cmd} (run ./dev.sh help)"
    ;;
esac
