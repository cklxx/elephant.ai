# 2026-01-09 - mock llm fallback missing api key or overrides

- Error: LLM responses came from `mock` even after setting env vars.
- Remediation: ensure `OPENAI_API_KEY` is set in the server process (canonical env names only) and clear managed overrides that force `llm_provider=mock` or empty `api_key`.
