# Plan: Execute top OKR slice (Calendar + Tasks tools)

## Goal
Start executing the most important OKR work by delivering a minimal Calendar + Tasks tool surface (query + create) with approval gating, aligned to the OKR-first roadmap.

## Scope
- In scope: Lark tool implementations + registry wiring + tests.
- Out of scope: full Lark API client refactor, docs/sheets/wiki integrations, and UI changes.

## Plan
- [completed] Define tool contracts for calendar query/create and task manage (list/create) with safe defaults.
- [completed] Implement Lark Calendar tools using Lark SDK v3.5.3.
- [completed] Implement Lark Task manage tool (list/create) with approval for write actions.
- [completed] Register new tools in tool registry.
- [completed] Add tests for argument validation and context requirements.
- [completed] Run lint/tests and restart dev services.
- [completed] Commit changes in incremental steps.
