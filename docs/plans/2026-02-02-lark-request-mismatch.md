# Plan: Diagnose Lark vs LLM request mismatch

Owner: cklxx-assistant
Date: 2026-02-02

## Goal
Identify why Lark message "我上一句是什么" becomes "你本地有哪些文件" in LLM request payload, and fix or explain the root cause.

## Steps
1. Gather evidence: locate Lark ingest/logging code and reproduce the request mapping path.
2. Trace message normalization/memory enrichment to see where content changes.
3. Add or adjust tests to cover correct payload mapping; fix bug if found.
4. Validate with full lint + tests; summarize findings.

## Status
- [x] Step 1: Gather evidence
- [x] Step 2: Trace normalization/memory enrichment
- [x] Step 3: Tests + fix
- [x] Step 4: Full lint + tests
