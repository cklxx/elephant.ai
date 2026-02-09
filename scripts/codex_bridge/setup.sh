#!/usr/bin/env bash
# Setup script for codex_bridge.
# No additional dependencies needed — codex_bridge.py uses only the stdlib.
# This script just verifies that `codex` is available.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Codex Bridge Setup ==="

if command -v codex &>/dev/null; then
    echo "codex found: $(command -v codex)"
    codex --version 2>/dev/null || true
else
    echo "WARNING: 'codex' not found in PATH."
    echo "Install it: npm install -g @openai/codex"
    exit 1
fi

echo "Setup complete. Bridge script: ${SCRIPT_DIR}/codex_bridge.py"
