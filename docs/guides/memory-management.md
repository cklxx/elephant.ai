# Memory Management

Updated: 2026-03-03

Strategy for loading, authoring, and maintaining the project's memory system.

---

## Always-Load Set

Load at every conversation start (~8 KB total):

1. `docs/memory/long-term.md` — stable cross-session rules.
2. `docs/guides/engineering-practices.md` — coding conventions.
3. Latest 3 **error summaries** from `docs/error-experience/summary/entries/` (by filename date DESC).
4. Latest 3 **good summaries** from `docs/good-experience/summary/entries/` (by filename date DESC).

No ranking algorithm needed — filenames are date-sorted; read the most recent.

---

## On-Demand Loading

Load when the task needs it:

| Source | Trigger |
|--------|---------|
| Full error/good entry | Summary lacks detail for the current task |
| `docs/memory/index.yaml` + `edges.yaml` | Need to search history by topic or find related entries |
| `docs/memory/tags.yaml` | Need tag-based filtering |
| `docs/postmortems/incidents/` | Task touches a component with a known incident |
| `docs/plans/` | Entering planning phase; need prior design references |
| 1-hop graph expansion | Already found a relevant entry; need its neighbors |

Retrieval rules:
- Summaries first; expand to full entry only when summary is insufficient.
- Prefer most recent item when multiple entries cover the same topic.
- Lark context: `memory_search → memory_get → memory_related → lark_chat_history`.

---

## Authoring Rules

### Long-Term Memory

- `docs/memory/long-term.md` = durable, long-lived lessons only.
- Update `Updated:` timestamp to hour precision (`YYYY-MM-DD HH:00`).
- Keep it concise — this file is loaded every session.

### Experience Entries

New entries must include a `## Metadata` block:

```yaml
id: <type>-YYYY-MM-DD-<short-slug>
type: error_entry | good_entry | error_summary | good_summary
date: YYYY-MM-DD
tags:
  - <tag>
links:
  related: []
  supersedes: []
  see_also: []
  derived_from: []
```

After editing memory docs, run: `go run ./scripts/memory/backfill_networked.go`.

### Index-Only Files

These files are indexes — never put content in them:
- `docs/error-experience.md`
- `docs/error-experience/summary.md`
- `docs/good-experience.md`
- `docs/good-experience/summary.md`

---

## Networked Memory Graph

### Node Types

`error_entry`, `error_summary`, `good_entry`, `good_summary`, `long_term`, `plan`

### Link Semantics

| Link | Meaning |
|------|---------|
| `related` | Peer concepts |
| `supersedes` | Newer replaces older |
| `see_also` | Supplementary reference |
| `derived_from` | Distilled from another entry |

### Index Artifacts

| File | Purpose |
|------|---------|
| `docs/memory/index.yaml` | Node registry — all entries |
| `docs/memory/edges.yaml` | Bidirectional link edges |
| `docs/memory/tags.yaml` | Controlled tag vocabulary |

---

## Topic Memory Files

Detailed notes organized by domain:

| File | Domain |
|------|--------|
| `docs/memory/long-term.md` | Cross-session stable rules |
| `docs/memory/kernel-ops.md` | Kernel operations patterns |
| `docs/memory/lark-devops.md` | Lark integration and DevOps |
| `docs/memory/eval-routing.md` | Eval and routing decisions |

---

## File Structure Summary

```
docs/
├── memory/
│   ├── long-term.md              # Always-load: stable rules
│   ├── index.yaml                # Node registry
│   ├── edges.yaml                # Link edges
│   ├── tags.yaml                 # Tag vocabulary
│   ├── kernel-ops.md             # Topic: kernel
│   ├── lark-devops.md            # Topic: lark/devops
│   ├── eval-routing.md           # Topic: eval/routing
│   └── networked/README.md       # Graph system docs
├── error-experience/
│   ├── entries/                   # Full error entries
│   └── summary/entries/           # Error summaries
├── good-experience/
│   ├── entries/                   # Full good entries
│   └── summary/entries/           # Good summaries
└── postmortems/
    ├── incidents/                 # Incident reports
    ├── templates/                 # Postmortem template
    └── checklists/                # Prevention checklist
```
