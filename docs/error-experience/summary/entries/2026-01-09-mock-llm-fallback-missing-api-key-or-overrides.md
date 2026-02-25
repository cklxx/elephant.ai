# 2026-01-09 - mock llm fallback missing api key or overrides

- Summary: LLM responses unexpectedly came from the mock provider due to missing API keys or override settings.
- Remediation: set `runtime.api_key` in `~/.alex/config.yaml` (or via `${OPENAI_API_KEY}`) and remove overrides that force `llm_provider=mock` or empty `api_key`.
