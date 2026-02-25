# Postmortems

Updated: 2026-02-25

## Purpose
- Provide a consistent incident-retrospective mechanism for regressions and high-impact design mistakes.
- Capture root cause, blast radius, fix, and concrete prevention actions.
- Ensure every incident produces both technical guardrails (tests/checks) and organizational memory (entries/summaries).

## Directory Layout
- `incidents/`: full incident postmortems (one file per incident)
- `templates/`: reusable templates
- `checklists/`: prevention and review checklists

## Required Workflow
1. Open an incident doc in `incidents/` within one working day.
2. Fill all mandatory sections from template.
3. Add an error-experience entry and summary entry.
4. Add at least one prevention action that is testable (test/lint/check/process gate).
5. Link implementation PR/commit and validation evidence.

## Naming Convention
- Incident file: `YYYY-MM-DD-<short-incident-slug>.md`
- Use stable, searchable words in slug (component + failure mode).

## Mandatory Fields (minimum)
- What happened
- Impact / blast radius
- Timeline (with exact dates)
- Root cause (technical + process)
- Fix implemented
- Prevention actions (owner + deadline + validation method)
- Evidence (tests/logs/commands)

