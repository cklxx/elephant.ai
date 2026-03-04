Summary: “四个 system prompt”来自同一 log_id 下的四次独立 LLM 调用（main/memory_extract/rephrase），不是单次请求内的核心系统提示词重复注入；排查时需先区分 request_id 级别与 session 聚合级别。

## Metadata
- id: errsum-2026-03-04-session-system-prompt-multi-call-misread
- tags: [summary, observability, prompt, lark]
- derived_from:
  - docs/error-experience/entries/2026-03-04-session-system-prompt-multi-call-misread.md
