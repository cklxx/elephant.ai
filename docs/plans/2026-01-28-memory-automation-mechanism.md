# Plan: Automated Memory Handling Mechanism (2026-01-28)

## Context
- Need an automatic mechanism to ingest, rank, and serve repo memory.
- Memory sources include error/good experience entries + summaries.
- Must support first-run load, progressive disclosure, and on-demand retrieval.

## Goals
- Deterministic, explainable ranking (recency/frequency/relevance).
- Incremental updates as new entries land.
- Minimal load on first run (top-K + summaries).
- Progressive disclosure with escalation triggers.

## Non-goals
- Global cross-repo memory sharing.
- Long-term archival beyond existing error/good summaries.

## Proposed Mechanism

### 1) Memory Indexer (offline + incremental)
- **Input folders**:
  - `docs/error-experience/entries/`
  - `docs/error-experience/summary/entries/`
  - `docs/good-experience/entries/`
  - `docs/good-experience/summary/entries/`
- **Parsing**:
  - Extract date from filename; parse `Error:`/`Summary:` + `Remediation:` lines.
  - Infer tags via lightweight keyword map + TF-IDF top n-grams.
- **Output artifacts** (YAML):
  - `docs/memory/index.yaml` (flat list with metadata and scores)
  - `docs/memory/clusters.yaml` (topic clusters + frequency)
  - `docs/memory/recent.yaml` (rolling recent window)

### 2) Ranking Model
- Score per item:
  - **Recency**: `exp(-(now - date)/half_life)`
  - **Frequency**: `log(1 + cluster_count)`
  - **Relevance**: lexical overlap + optional embedding cosine (if enabled)
- Combined:
  - `score = 0.5 * relevance + 0.3 * recency + 0.2 * frequency`
- Tie-breakers: recency → frequency → path

### 3) First-run Load (mandatory)
- Read latest 3–5 items from each folder, then merge with top-K from `index.yaml`.
- Keep **active set** size 8–12; store remainder as cold memory.
- Load summaries first; pull full entries only if summary lacks required detail.

### 4) Progressive Disclosure
- Escalate memory retrieval only when:
  - Test/error signature matches a known issue.
  - User request includes keywords mapped to a cluster.
  - Active set cannot answer (coverage gap).
- Expand in tiers: summary → full entry → related cluster.

### 5) Triggers & Automation
- **Indexer trigger**: on repo update / CI job / pre-commit hook.
- **Incremental**: reindex only files changed since last run.
- **Cache invalidation**: update `index.yaml` when any entry changes.

### 6) Traceability
- Each memory item includes:
  - `source_path`, `date`, `type` (error/good), `summary`, `remediation`, `tags`.
- When used, log: `memory_used: [ids]` in plan/progress notes.

## Plan
1. Add a memory automation spec (this doc) for review.
2. If approved, implement indexer + YAML artifacts + hooks.
3. Add tests for parsing + ranking + incremental updates.
4. Wire first-run load + progressive disclosure in agent runtime.

## Progress
- 2026-01-28: Plan created.
