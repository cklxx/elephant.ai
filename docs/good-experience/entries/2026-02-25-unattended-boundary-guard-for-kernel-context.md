# Use unattended boundary guard + dual-path tests for runtime-only prompt injections

## Context
- Feature: kernel alignment context in shared prompt assembly path.
- Risk: runtime-only payload can leak into user-facing channels when shared injection path has no boundary gate.

## What worked
1. Guard injection with runtime intent signal (`IsUnattendedContext`) at preparation boundary.
2. Add both negative and positive tests:
   - non-unattended must not inject.
   - unattended must inject.
3. Keep fix local to injection point to avoid broad behavior change.

## Why it worked
- The boundary signal already existed and represented the exact execution mode distinction.
- Dual-path tests encoded the contract and prevented silent regressions.
- Small-scope change reduced blast radius and review complexity.

## Reusable pattern
- For any context/prompt injector:
  1) define explicit applicability boundary,
  2) enforce in one entry point,
  3) validate with positive/negative tests.

## Metadata
- id: good-2026-02-25-unattended-boundary-guard
- tags: [good, pattern, prompt-injection, boundary, testing]
- links:
  - docs/postmortems/checklists/incident-prevention-checklist.md
  - docs/error-experience/entries/2026-02-25-kernel-alignment-context-leak-into-lark-session.md
