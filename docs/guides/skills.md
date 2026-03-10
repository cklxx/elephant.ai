# Skills Integration

> Last updated: 2026-03-10

How `alex` discovers and serves skills (Markdown playbooks).

## Skill Layout

Each skill lives in its own folder with a `SKILL.md` file. YAML frontmatter must include `name` and `description`:

```md
---
name: pdf_processing
description: Extract text and tables from PDFs.
---
# PDF Processing
...body...
```

Optional frontmatter for orchestration: `capabilities`, `governance_level` (`low|medium|high|critical`), `activation_mode` (`auto|semi_auto|manual`), `depends_on_skills`, `produces_events`, `requires_approval`.

## Discovery

Search order:
1. `ALEX_SKILLS_DIR` (if set)
2. `~/.alex/skills` (default)

When `ALEX_SKILLS_DIR` is unset, runtime syncs from repository `skills/` — copies missing skills only, never overwrites or deletes existing user skills.

Only folder-based `SKILL.md` layouts are supported. Missing frontmatter or duplicate names are rejected.

## Using the `skills` Tool

- `action=list` — catalog (names + descriptions)
- `action=show` — skill body
- `action=search` — ranked matches by name/description/body

## Dedup Policy

- One canonical entrypoint per capability domain.
- Remove thin wrappers when the canonical skill already covers the same user job.
- When skills overlap, merge the derived variant into the canonical name (e.g., `anygen-task-creator → anygen`).

## Security

- Prefer sandboxing for skill scripts.
- Only load trusted skills.
- Prompt before destructive actions.
- Log all executions.
