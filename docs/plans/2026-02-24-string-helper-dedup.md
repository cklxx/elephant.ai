# Plan: String Helper Deduplication Scan
Owner: cklxx
Date: 2026-02-24

## Goal
Inventory duplicated string normalization/clone/conversion helpers inside non-web `internal/` packages, assess low-risk behavior-preserving extractions, and recommend shared helpers plus test impact notes.

## Steps
- [x] Enumerated candidate helpers under `internal/` (excluding `web/`).
- [x] Grouped similar helpers by functionality and noted any existing shared utilities.
- [x] Identified low-risk duplicates worth factoring into shared helpers and scoped the extraction boundaries.
- [x] Summarized findings with target helper locations and estimated test impact.

## Progress
- Background task manager reimplements execution/autonomy normalization already covered by `internal/infra/executioncontrol`; a small refactor can just call that helper.
- Onboarding state normalization lives in both the store and the HTTP handler; exposing a shared `subscription.NormalizeOnboardingState` keeps behavior identical.
- Attachment normalization logic appears in both React and orchestration code paths; a single `ports.NormalizeAttachmentMap` can satisfy both callers without behavioral drift.
