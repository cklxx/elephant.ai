# 2026-01-27 - dev test blocked by rag embedder invalid OpenAI key

- Summary: `./dev.sh test` failed in `internal/rag` integration tests because `OPENAI_API_KEY` was a non-OpenAI `sk-kimi-...` key, causing 401 errors.
- Remediation: use a valid OpenAI key for `OPENAI_API_KEY`, or unset it / use a non-`sk-` prefix to skip the integration test.
- Resolution: not resolved in this run.
