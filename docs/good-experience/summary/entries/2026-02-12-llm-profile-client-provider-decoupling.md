# 2026-02-12 — Summary: LLM Profile + Client Provider Decoupling

- Unified runtime LLM config into `LLMProfile` and validated provider/key/base_url consistency at config boundary.
- Added app-layer `llmclient` helper so execution components receive ready-to-use clients from profile, without low-level API config assembly.
- Reduced tool registry coupling by removing unrelated LLM config fields from `toolregistry.Config`.
- Verified with full lint and full test suite (`go test ./...`).
