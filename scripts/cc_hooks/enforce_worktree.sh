#!/bin/bash
# enforce_worktree.sh — Claude Code PreToolUse hook that warns (but allows)
# file modifications on the main branch outside a git worktree.
#
# Previously this denied edits, but worktree enforcement added too much
# friction (shell cwd resets, stash/merge cycles for every small fix).
# A warning preserves the reminder without blocking flow.
#
# Usage: Configure as a PreToolUse hook in .claude/settings.json:
#
#   "PreToolUse": [{
#     "matcher": "Write|Edit",
#     "hooks": [{ "type": "command",
#       "command": "\"$CLAUDE_PROJECT_DIR\"/scripts/cc_hooks/enforce_worktree.sh",
#       "timeout": 5 }]
#   }]
#
# Environment variables:
#   WORKTREE_HOOK_DISABLE=1  Skip enforcement (escape hatch)
#
set -euo pipefail

# Escape hatch.
if [ "${WORKTREE_HOOK_DISABLE:-}" = "1" ]; then
  exit 0
fi

# Read hook event JSON from stdin.
INPUT=$(cat)

# Extract the working directory and file path from the hook payload.
CWD=$(echo "$INPUT" | jq -r '.cwd // "."')
FILE_PATH=$(echo "$INPUT" | jq -r '.tool_input.file_path // .tool_input.filePath // ""')

# Only enforce for files within this project. Cross-repo edits are not our concern.
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$CWD}"
if [ -n "$FILE_PATH" ] && [[ "$FILE_PATH" != "$PROJECT_DIR/"* ]]; then
  exit 0
fi

# Allowlist: never warn for these paths.
if [ -n "$FILE_PATH" ]; then
  # Normalize: if file_path is absolute, make it relative to CWD for pattern matching.
  REL_PATH="$FILE_PATH"
  if [[ "$FILE_PATH" == "$CWD/"* ]]; then
    REL_PATH="${FILE_PATH#"$CWD"/}"
  fi

  # Allow docs/, .claude/, and root-level *.md files.
  case "$REL_PATH" in
    docs/*|.claude/*)
      exit 0
      ;;
  esac

  # Root-level markdown files (no directory separator before .md).
  if [[ "$REL_PATH" != */* && "$REL_PATH" == *.md ]]; then
    exit 0
  fi
fi

# Determine which repo to check — prefer the file's repo over CWD.
if [ -n "$FILE_PATH" ] && [ -d "$(dirname "$FILE_PATH")" ]; then
  CHECK_DIR=$(dirname "$FILE_PATH")
else
  CHECK_DIR="$CWD"
fi

# Detect branch and worktree status.
BRANCH=$(git -C "$CHECK_DIR" branch --show-current 2>/dev/null || echo "")
if [ "$BRANCH" != "main" ]; then
  # Not on main — allow.
  exit 0
fi

GIT_DIR=$(git -C "$CHECK_DIR" rev-parse --git-dir 2>/dev/null || echo "")
GIT_COMMON=$(git -C "$CHECK_DIR" rev-parse --git-common-dir 2>/dev/null || echo "")

# Normalize paths for comparison.
GIT_DIR_REAL=$(cd "$CHECK_DIR" && realpath "$GIT_DIR" 2>/dev/null || echo "$GIT_DIR")
GIT_COMMON_REAL=$(cd "$CHECK_DIR" && realpath "$GIT_COMMON" 2>/dev/null || echo "$GIT_COMMON")

if [ "$GIT_DIR_REAL" != "$GIT_COMMON_REAL" ]; then
  # In a worktree — allow.
  exit 0
fi

# On main, not in a worktree — warn but allow.
echo "⚠️  Editing on main branch directly. Consider a worktree for multi-file changes." >&2
exit 0
