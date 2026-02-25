# Plan: Filter /model Listing to Usable Providers

## Goal
Ensure `/model list` only shows providers/models that are usable based on configured API keys (and reachable local servers), with tests updated accordingly.

## Steps
1. [x] Inspect current `/model` listing and catalog assembly to identify where providers without keys are included.
2. [x] Update catalog/model listing logic to filter out providers lacking configured credentials (keeping reachable local servers).
3. [x] Update/add tests to reflect the new filtering behavior and cover key scenarios.
4. [x] Run lint + full tests (ci-local ok; `make test` passes with OPENAI_API_KEY/LLM_API_KEY cleared).
