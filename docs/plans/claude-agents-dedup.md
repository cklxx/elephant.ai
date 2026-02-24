# Plan: Deduplicate CLAUDE.md and AGENTS.md

**Status**: Done
**Created**: 2026-02-24
**Approach**: Option A — AGENTS.md becomes a pure override file

## Problem

CLAUDE.md (175 lines) and AGENTS.md (98 lines) share ~60% content.
Duplicated items: worktree workflow (×4 across both files), TDD, lint/test, code review trigger, plan files, defensive code, memory loading.
Every rule change requires 2–4 edits across files; missed syncs produce contradictions.

## Changes

### CLAUDE.md (authoritative source)

1. **Merge §0 and §1.2**: §0 keeps identity/values only; all process rules move to §1.2. Eliminates 6 duplicate bullet points.
2. **Compress Project identity**: Remove "What proactive means" table (move to docs/ if needed). Keep 3-line summary + architecture diagram + package list.
3. **Actionable Design preferences**: Each preference gets a trigger condition and concrete behavior.
4. **Single worktree procedure**: One numbered list in §1.2. All other references say "follow worktree workflow (§1.2)".
5. **Conflict resolution meta-rule**: Add priority chain: safety > correctness > maintainability > speed.
6. **Memory loading**: Stays as-is (authoritative source).
7. **Rename Heuristic Prompting**: → "Proactive behavior constraints", rewritten as code-level constraints.

### AGENTS.md (override-only)

1. **Header**: "Base rules: see CLAUDE.md. This file contains additions and overrides only."
2. **Delete**: Project snapshot, Heuristic Prompting, Non-negotiables (all covered in CLAUDE.md), Memory loading.
3. **Keep (compressed)**: Code Review — 3 lines only (trigger condition, entry point, blocking rule). Detailed workflow stays in SKILL.md.
4. **Keep**: Any rules unique to AGENTS.md that don't exist in CLAUDE.md (subagent preference, architecture guardrails).

## Expected outcome

- CLAUDE.md: ~150 lines (from 175), zero duplicate rules internally
- AGENTS.md: ~25 lines (from 98), zero overlap with CLAUDE.md
- Total: ~175 lines (from 273), 36% reduction, zero information loss
