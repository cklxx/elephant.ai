# Plan: Conversation onboarding modal + model selector mapping

Owner: cklxx
Date: 2026-02-08

## Goal
Map the web surface area that should be patched to add a first-run onboarding modal on the conversation page, plus a provider/model selector driven by `/api/internal/subscription/catalog` and an onboarding state API, with minimal churn.

## Plan
1. Inspect conversation page and related UI entry points for where a modal can be mounted. (completed)
2. Locate existing model selection flow and subscription catalog usage to reuse. (completed)
3. Identify API/types/hooks modules for onboarding state + model catalog wiring. (completed)
4. Summarize patch targets and minimal-change strategy. (completed)

## Notes
- No code changes required for this mapping task.
