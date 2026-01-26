# Plan: TUI best-practices polish (2026-01-26)

## Goal
- Bring terminal-native chat output up to top-tier CLI UX quality with detail-focused improvements (layout, color, width handling, env controls).

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Research current CLI/TUI best practices (TTY detection, color rules, wrapping) and map gaps in current output.
2. Audit current CLI renderers and streaming output for width, color, and formatting consistency.
3. Implement targeted polish: color profile selection (NO_COLOR), terminal-width constraints for tool/status lines, and consistent rendering behavior.
4. Add/adjust tests for rendering width constraints and env-driven color handling.
5. Run `make fmt`, `make vet`, `make test`.

## Progress
- 2026-01-26: Plan created; engineering practices reviewed.
