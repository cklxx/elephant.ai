# Attention Gate Score Refactor

**Date:** 2026-03-10
**Status:** In Progress

## Goal

Refactor `internal/delivery/channels/lark/attention_gate.go` to classify messages with a numeric attention score (`0-100`) and derive five routing outcomes from configurable thresholds while preserving compatibility for existing urgency-based callers.

## Plan

1. Add score and routing primitives to the Lark attention gate and keep a compatibility path for existing `UrgencyLevel` callers.
2. Extend leader attention gate config defaults and validation with explicit routing thresholds.
3. Add focused unit tests for score calculation, threshold routing, compatibility mapping, and config validation.
4. Run formatting, targeted tests, lint, code review, and commit the worktree changes.
