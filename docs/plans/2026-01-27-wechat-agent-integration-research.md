# Plan: Research chatgpt-on-wechat integration for elephant.ai (2026-01-27)

## Goal
- Produce an actionable integration path to connect the elephant.ai agent with WeChat via chatgpt-on-wechat.
- Capture the integration mapping in a reusable research note.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Research chatgpt-on-wechat channels/config and LLM backend integration options.
2. Inspect elephant.ai server API surface and map it to the WeChat bridge.
3. Write a research report under `docs/research/` with steps and configuration mapping.
4. Run full lint + tests.
5. Commit changes.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Researched chatgpt-on-wechat configs and drafted the integration report.
- 2026-01-27: Updated the report with upstream config details and integration mapping.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (both failed: cmd/alex context import errors).
- 2026-01-27: Logged error-experience entry + summary for the lint/test failures.
- 2026-01-27: Dug into CoW WeChat channels (itchat `wx`, wechaty `wxy`) and noted wechatferry mention in the upstream changelog.
- 2026-01-27: Refined the research report with personal account channel notes; ran `./dev.sh lint` and `./dev.sh test`.
