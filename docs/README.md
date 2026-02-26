# elephant.ai Documentation Portal
> Last updated: 2026-02-26

Start here for current architecture, config, and operations docs.

## Core Docs

- [`../AGENTS.md`](../AGENTS.md): Repo workflow and safety constraints.
- [`reference/ARCHITECTURE_AGENT_FLOW.md`](reference/ARCHITECTURE_AGENT_FLOW.md): Runtime layering and execution flow.
- [`reference/CONFIG.md`](reference/CONFIG.md): Canonical config schema and precedence.
- [`guides/quickstart.md`](guides/quickstart.md): Local setup and dev loop.
- [`operations/DEPLOYMENT.md`](operations/DEPLOYMENT.md): Local/Compose/K8s deployment guidance.

## Doc Types

- **Non-record (living) docs**: `docs/reference/`, `docs/guides/`, `docs/operations/`, top-level indexes and README docs.
- **Record docs**: `docs/plans/`, `docs/research/`, `docs/analysis/`, `docs/reviews/`, `docs/error-experience/`, `docs/good-experience/`, `docs/postmortems/`.

When behavior/config/architecture changes, prioritize updating non-record docs first.

## Naming Convention

- New files: **kebab-case** (`kernel-deep-dive.md`).
- Existing UPPER_SNAKE (`REFACTOR_LEDGER.md`) and snake_case (`evaluation_set.md`) are kept as-is to avoid broken links.

## Directory Indexes

- [`reference/README.md`](reference/README.md) — Architecture, config, subsystem reference
- [`guides/README.md`](guides/README.md) — Setup and workflow guides
- [`operations/README.md`](operations/README.md) — Deployment, tooling, E2E specs
- [`plans/README.md`](plans/README.md) — Implementation plans
- [`research/README.md`](research/README.md) — Research reports and benchmarks
- [`analysis/README.md`](analysis/README.md) — Evaluation reports and project analysis
- [`memory/README.md`](memory/README.md) — Long-term memory and topic files
- [`error-experience.md`](error-experience.md) — Error experience index
- [`good-experience.md`](good-experience.md) — Good experience index
- [`postmortems/README.md`](postmortems/README.md) — Incident postmortems
