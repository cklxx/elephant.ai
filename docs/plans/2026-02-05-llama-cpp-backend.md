# Plan: llama.cpp backend + GGUF weight downloader

Owner: cklxx (requested)
Status: in_progress
Updated: 2026-02-05

## Goal

- Add a first-class `llama.cpp` LLM provider (OpenAI-compatible HTTP API; typically `llama-server`).
- Provide an automated way to download quantized GGUF weights (Hugging Face) for local inference workflows.

## Non-goals (for this iteration)

- Implement a full llama.cpp runtime embedded in Go (cgo bindings).
- Provide a full model catalog / automatic quant selection.
- GPU-specific tuning (we’ll keep defaults and allow passing extra args).

## Proposed UX

### 1) Use llama.cpp backend

- Set:
  - `runtime.llm_provider: "llama.cpp"`
  - `runtime.base_url: "http://127.0.0.1:8080/v1"` (or leave empty to use the provider default)
  - `runtime.llm_model: "<any label>"` (passed as `model` in OpenAI-compatible request)

### 2) Download quantized weights

- CLI:
  - `alex llama-cpp pull <hf_repo> <gguf_file> [--revision <rev>] [--dir <dest>] [--sha256 <hex>]`
- Default download dir:
  - `~/.alex/models/llama.cpp/hf/<repo>/<revision>/<file>`

## Implementation outline

1. `internal/llm`: add `llamacpp_client.go` (OpenAI-compatible, default base URL `http://127.0.0.1:8080/v1`).
2. Config safety:
   - Treat `llama.cpp` as a keyless provider (like `ollama`) in config readiness + server bootstrap.
   - Extend `alex model use` to clear `api_key` / `base_url` when switching to local providers.
3. `internal/llamacpp`: add a small HF downloader with atomic writes and optional SHA256 check.
4. `cmd/alex`: add `alex llama-cpp pull ...` command.
5. Tests:
   - Downloader URL building + path layout + SHA mismatch.
   - Factory accepts provider aliases and returns a client.
6. Docs:
   - `docs/reference/CONFIG.md` and `.env.example` updates (YAML examples only).

## Rollout / Risks

- Large file downloads: use a temp file + rename for atomicity; avoid partial corruption.
- Credentials: do not require an API key for `llama.cpp`; avoid accidentally sending remote API keys to local servers by clearing `api_key` on local selection.

