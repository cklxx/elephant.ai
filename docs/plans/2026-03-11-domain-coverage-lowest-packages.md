# Domain Coverage Lowest Packages

**Date:** 2026-03-11
**Status:** In Progress

## Goal

Run `go test -cover ./internal/domain/...`, identify the three lowest-coverage packages under `internal/domain`, add key-path tests for each, then validate and commit the changes.

## Plan

1. Measure current coverage for `./internal/domain/...` and rank packages by coverage.
2. Inspect the weakest three packages and their nearby test patterns.
3. Add focused tests for the main success and edge paths that are currently untested.
4. Run targeted tests, rerun domain coverage, run lint/code review, and commit the scoped changes.
