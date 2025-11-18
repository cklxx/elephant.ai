# ALEX Documentation Portal
> Last updated: 2025-11-18


This repository ships with a large collection of docs covering architecture, operations, research, and design. Use this page as the single entry point to discover what you need.

---

## 🗂️ Document Families

| Directory | Purpose | Representative Topics |
|-----------|---------|------------------------|
| [`architecture/`](architecture/) | Deep dives into system design decisions and large-scale diagrams. | Sandbox migration, sprint architecture summaries, web UI service design. |
| [`reference/`](reference/) | Authoritative specifications and API references. | Agent overview, presets, MCP integration, formatting, cost tracking, observability. |
| [`guides/`](guides/) | Task-focused walkthroughs and quick-starts. | SSE streaming, acceptance tests, server operation. |
| [`operations/`](operations/) | Day-two operations and release engineering. | Deployment, monitoring, publishing, release processes. |
| [`design/`](design/) | UX and interaction design notes. | TUI/CLI patterns, output formatting, dashboard behaviour. |
| [`analysis/`](analysis/) | Research reports and architectural assessments. | Competitive landscape, AI coding studies, alignment reports. |
| [`planning/`](planning/) & [`sprints/`](sprints/) | Historical plans and sprint retrospectives. | Roadmaps, feature prioritisation, sprint architecture notes. |
| [`research/`](research/) | Exploratory investigations and experiments. | Advanced agent capabilities, deep search, tooling experiments. |
| [`web/`](web/) | Frontend-specific docs. | Component guides, SSE integration for the dashboard. |
| [`assets/`](assets/) & [`diagrams/`](diagrams/) | Shared visuals and Mermaid diagrams. | System overviews, data flow, ReAct cycle illustrations. |

The standalone [`AGENT.md`](AGENT.md) document provides an end-to-end explanation of the agent runtime and should be your first stop before diving deeper.

---

## 🔎 Finding the Right Document

1. **Understanding how the agent works?** Start with [`AGENT.md`](AGENT.md) and then explore the architecture deep dives.
2. **Configuring or extending capabilities?** See the references for presets, tools, MCP, and observability.
3. **Running or deploying the system?** Visit the guides and operations folders for CLI commands, server setup, and monitoring.
4. **Researching past decisions?** Browse the analysis, planning, and sprint folders for historical context.
5. **Working on the dashboard?** Use the `web/` docs along with `design/` for UX decisions.

Each document follows the principle: **clear, concise, and actionable**. If you add new material, update this index with a short description so future contributors can discover it easily.

---

## 🧹 Documentation Maintenance

- **2025-11-18** – Removed the unrelated `research/love_sociological_analysis.md` file and added a `Last updated` stamp to every doc so contributors know when material was reviewed.
- Removed stale TODO markers from long-lived design docs and replaced them with current terminology.
- Updated the sandbox migration checklist to reflect its completed status with an explicit maintenance note.
- Clarified TUI design snippets so configuration-derived values are referenced directly.
- Replaced the todo tool transcript dump with a structured reference covering usage patterns and schemas.

Keep this section updated whenever you perform broad documentation clean-ups so contributors can see what changed at a glance.

---

## 🧭 Contribution Tips

- Keep references authoritative and up to date with the codebase.
- Link related docs across directories to avoid duplication.
- Prefer Markdown tables, callouts, and diagrams for complex flows.
- When proposing significant changes, add or update diagrams in `diagrams/` and cite them from the relevant docs.

For contribution and style guidance refer to [`docs/reference/FORMATTING_GUIDE.md`](reference/FORMATTING_GUIDE.md).
