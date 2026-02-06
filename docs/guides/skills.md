# Skills integration
> Last updated: 2025-02-05

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

## Discovery
`alex` searches for skills in this order:
1) `ALEX_SKILLS_DIR` (absolute or relative path)
2) `~/.alex/skills`

When `ALEX_SKILLS_DIR` is not set, runtime and web catalog generation both use `~/.alex/skills` and run a one-way sync from repository `skills/`:
- copy only missing skill directories to `~/.alex/skills`
- never overwrite existing user skills with the same name
- never delete files from `~/.alex/skills`

Only folder-based `SKILL.md` layouts are supported. Skills with missing frontmatter are rejected; duplicate names are rejected.

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

## Security considerations
Running skill-bundled scripts can be risky. Prefer:
- **Sandboxing**: execute scripts in isolated environments.
- **Allowlisting**: load only trusted skills.
- **Confirmation**: prompt before potentially destructive actions.
- **Logging**: record executions for auditability.
