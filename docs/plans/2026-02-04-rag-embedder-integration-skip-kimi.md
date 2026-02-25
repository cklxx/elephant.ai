# Plan: Skip OpenAI Embedder Integration Test for `sk-kimi` Keys

## Status: Completed
## Date: 2026-02-04

## Problem
`./dev.sh test` loads `.env` and exports `OPENAI_API_KEY`. When `OPENAI_API_KEY` is a Moonshot/Kimi key (`sk-kimi-...`), `internal/rag`'s `TestEmbedder_Integration` runs against `https://api.openai.com/v1` and fails with 401, breaking the FAST/SLOW gate.

## Plan
1. Make `TestEmbedder_Integration` skip when `OPENAI_API_KEY` is an `sk-kimi-...` key.
2. Run `./dev.sh lint` and `./dev.sh test` to confirm CI-parity passes.

## Progress
- [x] Skip `sk-kimi-...` keys in `internal/rag/embedder_test.go`.
- [x] Run `./dev.sh lint`.
- [x] Run `./dev.sh test`.

