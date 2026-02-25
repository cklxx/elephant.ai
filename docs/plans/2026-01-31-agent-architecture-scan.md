# Plan: Agent Architecture / Events / Memory / RAG Scan

## Goal
Review agent architecture, event system, memory system, and RAG implementation; identify issues and provide recommendations.

## Steps
- [x] Locate core architecture docs and code entry points for agent, events, memory, and RAG.
- [x] Trace event flow (ingest → partition → display/record) and identify coupling or correctness risks.
- [x] Review memory system boundaries and data flow (capture → storage → retrieval → injection).
- [x] Review RAG pipeline (retrieval sources, scoring, filtering, injection) and failure modes.
- [x] Summarize findings with prioritized issues and actionable recommendations.

## Notes
- Follow engineering practices; prefer maintainable changes.
- Keep agent/ports free of memory/RAG deps.
