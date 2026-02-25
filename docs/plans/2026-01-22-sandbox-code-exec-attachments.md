# Sandbox Code Execution + Attachment Retrieval/CDN Upload

**Goal:** Add sandbox code execution tooling and ensure sandbox-originated attachments are fetched from the sandbox and normalized via CDN upload where configured.

## System View
- Sandbox tools run against AIO Sandbox via `/v1/*` APIs with session pinning.
- Attachments stay inline in agent state; CDN externalization happens at boundaries or in tool-specific uploaders (e.g., Seedream).
- We need sandbox-side file retrieval to bring outputs back, and a consistent upload path when attachments are produced.

## Plan
1) Add sandbox attachment retrieval helpers (download from sandbox + optional CDN normalization via attachment migrator).
2) Extend sandbox tools that produce attachments to run the uploader (browser screenshots) and allow file-based outputs for shell/code tools.
3) Introduce `sandbox_code_execute` built on sandbox shell + file APIs.
4) Update tool presets and add focused unit tests.
5) Run full lint/tests and log progress.

## Progress Log
- 2026-01-22: Plan created.
- 2026-01-22: Added sandbox attachment helpers, uploader wiring, and new sandbox_code_execute tool; updated presets and formatter.
- 2026-01-22: Ran ./dev.sh lint and ./dev.sh test (happy-dom AbortError logs still emitted).
