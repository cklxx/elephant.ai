# Codex External Agent Default Model (ChatGPT Subscription)

**Date:** 2026-02-03
**Status:** Complete

## Problem

`runtime.external_agents.codex.default_model` defaults to `"o3"` in:
- `internal/config/types.go` (baseline defaults)
- `configs/config.yaml` (example config)
- `docs/reference/external-agents-codex-claude-code.md` (setup guide)

When Codex CLI is authenticated via ChatGPT subscription (OAuth in `~/.codex/auth.json`),
requests with `model="o3"` fail immediately:

> The 'o3' model is not supported when using Codex with a ChatGPT account.

This makes `bg_dispatch(agent_type=codex)` fail out-of-the-box for subscription users.

## Goal

Make the default external-agent Codex setup work for ChatGPT subscription sign-in without
requiring users to override `default_model`, while keeping the model configurable.

## Plan

1) Add a regression test asserting the default Codex model is subscription-compatible.
2) Switch the baseline default model to a Codex-supported model (e.g. `gpt-5-codex`).
3) Update `configs/config.yaml` and docs to match the new default.
4) Run full lint + tests; ship as small incremental commits.

## Done

- Default model changed from `o3` → `gpt-5-codex` (code + example config + docs).
- Regression test added to pin the default.
- Error-experience entry + summary captured for the incident.
