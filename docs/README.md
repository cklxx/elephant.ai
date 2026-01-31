# ALEX Documentation Portal
> Last updated: 2026-01-31

This landing page highlights the docs you are most likely to open in a typical week. Start with the agent overview, then use quickstart or deployment for hands-on work.

---

## 📌 Core Docs

- [`../AGENTS.md`](../AGENTS.md): Repo agent workflow, safety rules, and prompt posture.
- [`reference/ARCHITECTURE_AGENT_FLOW.md`](reference/ARCHITECTURE_AGENT_FLOW.md): Architecture and execution flow overview across delivery surfaces.
- [`reference/CONFIG.md`](reference/CONFIG.md): Canonical configuration schema, merge precedence, and annotated examples.
- [`guides/quickstart.md`](guides/quickstart.md): Fast path to build and run the agent locally with the minimum required steps.
- [`operations/DEPLOYMENT.md`](operations/DEPLOYMENT.md): Deployment guide for local runs, Docker Compose, and Kubernetes clusters.

Use these in order when you need a fast answer:

1. **How does the agent think and act?** Check `../AGENTS.md`, then `reference/ARCHITECTURE_AGENT_FLOW.md`.
2. **How do I develop or troubleshoot?** Check `reference/ARCHITECTURE_AGENT_FLOW.md` (architecture) or `guides/quickstart.md` (hands-on).
3. **How do I configure or deploy?** Check `reference/CONFIG.md` for knobs and `operations/DEPLOYMENT.md` for runtime setups.

If you add new material, keep this list short and focused on docs that people reach for weekly. Remove or archive anything that drifts from that bar.

---

## Indexes

- [`reference/README.md`](reference/README.md)
- [`guides/README.md`](guides/README.md)
- [`operations/README.md`](operations/README.md)
- [`plans/README.md`](plans/README.md)
- [`research/README.md`](research/README.md)
- [`memory/README.md`](memory/README.md)
- [`error-experience.md`](error-experience.md)
- [`good-experience.md`](good-experience.md)

## Top-level Docs

- [`REFACTOR_LEDGER.md`](REFACTOR_LEDGER.md)
- [`evaluation_remaining_features.md`](evaluation_remaining_features.md)
- [`evaluation_status_and_todo.md`](evaluation_status_and_todo.md)
- [`frontend-best-practices-research.md`](frontend-best-practices-research.md)

---

## 🧹 Maintenance Log

- **2026-01-31** – Added directory indexes and clarified top-level doc navigation.
- **2025-12-26** – Refreshed the homepage descriptions and navigation pointers to reflect the current core doc set.
- **2025-11-29** – Archived most historical analysis, design, and research docs to keep the portal concise; retained only the core reference, quickstart, and deployment guides.
- **2025-11-18** – Added `Last updated` stamps to every doc and refreshed TUI design notes.
