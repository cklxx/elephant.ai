# Skills integration
> Last updated: 2026-03-05

This guide explains how `alex` discovers and serves skills (Markdown playbooks) following the [Agent Skills specification](https://agentskills.io/integrate-skills).

## Skill layout
- Each skill lives in its own folder with a `SKILL.md` (or `SKILL.mdx`) file.
- YAML frontmatter must include `name` and `description`:

```md
---
name: pdf_processing
description: Extract text and tables from PDFs.
---
# PDF Processing
...body...
```

Optional frontmatter for meta orchestration:
- `capabilities`: declarative capability tags (for example `lark_chat`, `self_schedule`, `self_evolve_soul`)
- `governance_level`: `low|medium|high|critical`
- `activation_mode`: `auto|semi_auto|manual`
- `depends_on_skills`: activation dependencies by skill name
- `produces_events`: declared event names emitted by the skill
- `requires_approval`: whether policy should gate auto activation

## Discovery
`alex` searches for skills in this order:
1) `ALEX_SKILLS_DIR` (absolute or relative path)
2) `~/.alex/skills`

When `ALEX_SKILLS_DIR` is not set, runtime and web catalog generation both use `~/.alex/skills` and run a one-way sync from repository `skills/`:
- copy only missing skill directories to `~/.alex/skills`
- never overwrite existing user skills with the same name
- never delete files from `~/.alex/skills`

Only folder-based `SKILL.md` layouts are supported. Skills with missing frontmatter are rejected; duplicate names are rejected.

## Product-level dedup policy
- Evaluate skill redundancy by **user-visible capability coverage**, not implementation differences.
- Prefer **one canonical entrypoint per product capability domain** (for example, keep `feishu-cli` as the single Feishu operations skill).
- Remove thin wrapper skills when the canonical entrypoint already supports the same user job-to-be-done.
- Keep domain workflow skills only if they provide clear product value beyond generic model behavior (for example, stronger SOP, governance, or artifacts).
- For capability review docs and decisions, list overlaps in terms of user intents, expected outcomes, and operating surface area.
- When two skills overlap, merge the more specific/derived variant into the canonical product-facing skill name (for example, `anygen-task-creator -> anygen`), not the reverse.

## Prompt metadata
At startup we parse frontmatter to build a compact catalog for prompts and the `skills` tool. The system prompt injects the metadata using the Agent Skills `<available_skills>` XML format. A Claude-style example based on the Agent Skills guide:

```xml
<available_skills>
  <skill>
    <name>pdf_processing</name>
    <description>Extracts text and tables from PDF files.</description>
    <location>/path/to/skills/pdf-processing/SKILL.md</location>
  </skill>
  <skill>
    <name>data_analysis</name>
    <description>Analyzes datasets, generates charts, and creates summary reports.</description>
    <location>/path/to/skills/data-analysis/SKILL.md</location>
  </skill>
</available_skills>
```

## Using the `skills` tool
- `action=list` renders the catalog (names + descriptions).
- `action=show` returns a specific skill body.
- `action=search` ranks matches by name/description/body.

## Meta orchestration policy
- Runtime can apply additional activation and linkage rules from `configs/skills/meta-orchestrator.yaml`.
- Policy controls activation defaults, governance gates, immutable SOUL sections, and skill linkage edges.
- Prompt injection includes a compact orchestration summary (`Meta Skill Orchestration`) when enabled.

## Security considerations
Running skill-bundled scripts can be risky. Prefer:
- **Sandboxing**: execute scripts in isolated environments.
- **Allowlisting**: load only trusted skills.
- **Confirmation**: prompt before potentially destructive actions.
- **Logging**: record executions for auditability.
