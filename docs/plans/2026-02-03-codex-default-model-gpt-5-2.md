# Codex External Agent Default Model → gpt-5.2-codex

**Date:** 2026-02-03
**Status:** Complete

## Problem

The external-agent Codex default model needed to move to `gpt-5.2-codex` to align with
ChatGPT subscription sign-in defaults, while keeping compatibility notes intact.

## Goal

Set `runtime.external_agents.codex.default_model` to `gpt-5.2-codex` across defaults,
example config, docs, and tests, plus update the prior error-experience remediation
to reflect the new default.

## Plan

1) Update the regression test to expect `gpt-5.2-codex` (TDD).
2) Switch defaults and config example to `gpt-5.2-codex`.
3) Update docs note + remediation entries to mention `gpt-5.2-codex`.
4) Run full lint + tests; commit in small increments; merge back to `main`.

## Done

- Default model updated to `gpt-5.2-codex` (code + example config + docs).
- Regression test updated to assert the new default.
- Error-experience remediation updated to reflect the new default.
