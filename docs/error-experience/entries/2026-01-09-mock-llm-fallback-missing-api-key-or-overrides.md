# 2026-01-09 - mock llm fallback missing api key or overrides

- Error: LLM responses came from `mock` even after updating configuration.
- Remediation: ensure `runtime.api_key` is set in `~/.alex/config.yaml` (or interpolated via `${OPENAI_API_KEY}`) and clear overrides that force `llm_provider=mock` or empty `api_key` under `overrides`.
