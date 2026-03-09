#!/usr/bin/env bash
# diag.sh — One-shot diagnostics collector for alex-server.
# Connects to the debug HTTP server (default :9090) and dumps heap, goroutine,
# allocs, and CPU profiles into a timestamped directory.
#
# Usage:
#   scripts/diag.sh [port]
#
# Example:
#   scripts/diag.sh          # default port 9090
#   scripts/diag.sh 9091     # custom port

set -euo pipefail

PORT="${1:-9090}"
BASE_URL="http://localhost:${PORT}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
DUMP_DIR="${ALEX_LOG_DIR:-logs}/diag-${TIMESTAMP}"

mkdir -p "${DUMP_DIR}"

echo "=== alex-server diagnostics ==="
echo "Target:    ${BASE_URL}"
echo "Output:    ${DUMP_DIR}"
echo ""

# Health check first.
if ! curl -sf "${BASE_URL}/health" > /dev/null 2>&1; then
    echo "ERROR: Cannot reach ${BASE_URL}/health — is alex-server running?"
    exit 1
fi

echo "[1/5] Heap profile..."
curl -sf "${BASE_URL}/debug/pprof/heap" > "${DUMP_DIR}/heap.prof" 2>/dev/null && \
    echo "      -> ${DUMP_DIR}/heap.prof" || echo "      FAILED"

echo "[2/5] Goroutine profile (debug=1)..."
curl -sf "${BASE_URL}/debug/pprof/goroutine?debug=1" > "${DUMP_DIR}/goroutine.txt" 2>/dev/null && \
    echo "      -> ${DUMP_DIR}/goroutine.txt" || echo "      FAILED"

echo "[3/5] Goroutine profile (binary)..."
curl -sf "${BASE_URL}/debug/pprof/goroutine" > "${DUMP_DIR}/goroutine.prof" 2>/dev/null && \
    echo "      -> ${DUMP_DIR}/goroutine.prof" || echo "      FAILED"

echo "[4/5] Allocs profile..."
curl -sf "${BASE_URL}/debug/pprof/allocs" > "${DUMP_DIR}/allocs.prof" 2>/dev/null && \
    echo "      -> ${DUMP_DIR}/allocs.prof" || echo "      FAILED"

CPU_SECONDS="${DIAG_CPU_SECONDS:-10}"
echo "[5/5] CPU profile (${CPU_SECONDS}s)..."
curl -sf "${BASE_URL}/debug/pprof/profile?seconds=${CPU_SECONDS}" > "${DUMP_DIR}/cpu.prof" 2>/dev/null && \
    echo "      -> ${DUMP_DIR}/cpu.prof" || echo "      FAILED"

echo ""
echo "=== Summary ==="

# Quick goroutine count from debug output.
if [ -f "${DUMP_DIR}/goroutine.txt" ]; then
    GOROUTINE_COUNT=$(head -1 "${DUMP_DIR}/goroutine.txt" | grep -oE '[0-9]+' | head -1 || echo "?")
    echo "Goroutines:  ${GOROUTINE_COUNT}"
fi

echo "Files:"
ls -lh "${DUMP_DIR}/"

echo ""
echo "To analyze:"
echo "  go tool pprof ${DUMP_DIR}/heap.prof"
echo "  go tool pprof ${DUMP_DIR}/cpu.prof"
echo "  go tool pprof -http=:8888 ${DUMP_DIR}/heap.prof   # web UI"
