# 2026-01-27 - dev test blocked by rag embedder invalid OpenAI key

## Error
- `./dev.sh test` failed in `internal/rag` (`TestEmbedder_Integration`) with `API error 401` because `OPENAI_API_KEY` points to a non-OpenAI key (`sk-kimi-...`).

## Impact
- Full test validation cannot complete because integration tests hit the OpenAI endpoint with invalid credentials.

## Notes / Suspected Causes
- The integration test only checks that the key begins with `sk-`, so non-OpenAI providers that reuse the prefix are not skipped.

## Remediation Ideas
- Set `OPENAI_API_KEY` to a valid OpenAI key before running `./dev.sh test`.
- Alternatively, unset `OPENAI_API_KEY` (or set it to a non-`sk-` prefix) to skip the integration test.

## Resolution (This Run)
- Not resolved; tests failed due to invalid API key.

## Follow-up (2026-02-04)
- `TestEmbedder_Integration` now skips `sk-kimi-...` keys so `./dev.sh test` (CI parity) doesn't accidentally run OpenAI integration calls with non-OpenAI credentials.
