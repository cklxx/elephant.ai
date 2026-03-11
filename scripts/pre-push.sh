#!/usr/bin/env bash
# shellcheck shell=bash
# Compatibility wrapper for the unified CI entrypoint.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "${SCRIPT_DIR}/ci-check.sh" pre-push "$@"
