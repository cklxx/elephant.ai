# Main Last 15 Commits Review

Reviewed with `git log --oneline -15` and `git show` for each commit on `main`.

## Executive Summary

- Material findings in 4 of 15 commits:
  - `75e21d81`: config schema validation was implemented but not wired into startup.
  - `6fa47784`: public `/health` now leaks per-model internals and raw upstream errors.
  - `759d3d17`: the new Lark deduper is race-prone under concurrent first delivery.
  - `4fb4e61d`: memory cleanup loop leaks because it is started with `context.Background()` and never cancelled.
- The remaining commits look reasonable from a static review perspective. The main residual risk is test depth on large refactors.

---

## 371af394 `refactor(lark): split gateway.go (1084 lines) into 6 files by responsibility`

- Code quality & correctness:
  - No material semantic regression stood out. The diff is a file split/extraction and the moved logic remains consistent.
- Test sufficiency:
  - No commit-local tests were added. That is acceptable for a mechanical refactor only if the pre-existing Lark gateway suite was run.
- Security concerns or bugs:
  - No new security issue identified.

## 5207e049 `refactor(react): split background.go (1537→6 files, max 351 lines)`

- Code quality & correctness:
  - No obvious behavior change in the reviewed hunks; this looks like a structural split.
- Test sufficiency:
  - No targeted tests were added for the refactor itself, so confidence depends on the existing ReAct/background-task test suite.
- Security concerns or bugs:
  - No new security issue identified.

## da918db5 `fix(bootstrap): migrate os.Getenv("ALEX_LOG_DIR") to config loader`

- Code quality & correctness:
  - The change is small and correct: watchdog dump path now comes from bootstrap config instead of direct `os.Getenv`.
- Test sufficiency:
  - No test changes in the commit, but the change is narrow and the existing getenv-guard test mentioned in the message likely covers it.
- Security concerns or bugs:
  - No new security issue identified.

## 046eb8f9 `fix(context): preserve base snapshot during replay truncation`

- Code quality & correctness:
  - The fix is correct and addresses the failure mode described in the commit message. Preserving the first snapshot while keeping the newest `N-1` snapshots is the right behavior for replay continuity.
- Test sufficiency:
  - Adequate for the change. The updated history-manager test covers the intended truncation invariant.
- Security concerns or bugs:
  - No new security issue identified.

## 820b0941 `fix(arch): resolve 2 layer policy violations`

- Code quality & correctness:
  - The move from delivery-layer sanitization to `internal/shared/errsanitize` is clean. The timer change to `id.MarkUnattendedContext` also looks correct.
- Test sufficiency:
  - Good for the sanitizer move: the full sanitizer test suite moved with the implementation.
- Security concerns or bugs:
  - No new security issue identified.

## 8e2fff74 `feat(lark): increase streaming block max chunks from 5 to 8`

- Code quality & correctness:
  - Straightforward constant change. The behavior change matches the commit intent.
- Test sufficiency:
  - Sufficient for the scope. `message_splitter_test.go` was updated to keep the max-chunk behavior covered.
- Security concerns or bugs:
  - No new security issue identified.

## 04bf9e3f `feat(react): precise LLM token usage tracking across ReAct iterations`

- Code quality & correctness:
  - The accumulation path is coherent: usage is recorded after successful LLM responses and copied into `TaskResult`.
  - `TokensUsed` intentionally remains the estimated context-window count, while `TokenBreakdown` carries actual provider-reported usage. That split is defensible, just easy to misuse later.
- Test sufficiency:
  - Good unit coverage for single-call, multi-call, and zero-usage accumulation.
  - Missing integration coverage for downstream consumers/adapters that may eventually need `TokenBreakdown`.
- Security concerns or bugs:
  - No new security issue identified.

## 4fb4e61d `feat(memory): add automatic expiration cleanup for daily entries`

- Code quality & correctness:
  - `P1`: the cleanup goroutine is not bound to container lifetime. `Build()` now calls `buildMemoryEngine(context.Background())` in [`internal/app/di/container_builder.go:67`](../internal/app/di/container_builder.go#L67), and `buildMemoryEngine` immediately starts `StartCleanupLoop` in [`internal/app/di/builder_session.go:25`](../internal/app/di/builder_session.go#L25). The loop exits only when its context is cancelled in [`internal/infra/memory/cleanup.go:96`](../internal/infra/memory/cleanup.go#L96), but `context.Background()` is never cancelled and `MarkdownEngine.Drain` does not stop it. Rebuilding containers leaks background goroutines.
- Test sufficiency:
  - The archival logic is well covered.
  - Missing lifecycle tests: no test covers cancellation, repeated container builds, or shutdown behavior.
- Security concerns or bugs:
  - No direct security issue. The main risk is goroutine/resource leakage over long-lived processes and tests.

## 759d3d17 `feat(lark): add event dedup with message_id + event_id and TTL cleanup`

- Code quality & correctness:
  - `P1`: `eventDedup.isDuplicate` performs a non-atomic `Load`/`Store` sequence on `sync.Map` in [`internal/delivery/channels/lark/dedup.go:56`](../internal/delivery/channels/lark/dedup.go#L56). Two concurrent first deliveries of the same `(message_id,event_id)` can both observe a miss and both proceed, so duplicate task execution is still possible under concurrent redelivery. The old implementation used a mutex around the check-and-record path; this one does not.
- Test sufficiency:
  - `TestEventDedup_ConcurrencySafety` in [`internal/delivery/channels/lark/dedup_test.go:94`](../internal/delivery/channels/lark/dedup_test.go#L94) does not assert a single winner for the shared key; it only checks that the key is duplicate after the storm, so it misses the race.
  - `TestEventDedup_CleanupGoroutine` in [`internal/delivery/channels/lark/dedup_test.go:120`](../internal/delivery/channels/lark/dedup_test.go#L120) never shortens the 1-minute cleanup ticker, so it does not actually verify that the background sweep ran.
- Security concerns or bugs:
  - No direct secret leak, but duplicate message execution can duplicate outbound replies and repeated tool side effects.

## 667052f1 `feat(coordinator): add structured step timing to ExecuteTask`

- Code quality & correctness:
  - No correctness issue found. The helper is simple and isolated.
  - The logging is “structured-like” rather than actual structured fields, but that is a quality preference rather than a bug.
- Test sufficiency:
  - Helper-level coverage is fine.
  - No end-to-end test proves that `ExecuteTask` emits the new timing lines in the real orchestration path.
- Security concerns or bugs:
  - No new security issue identified.

## 75e21d81 `feat(config): add JSON Schema validation for config.yaml`

- Code quality & correctness:
  - `P1`: the new validator is not wired into startup. `ValidateConfigSchema` exists in [`internal/shared/config/validate_schema.go:18`](../internal/shared/config/validate_schema.go#L18), but repository search shows no production call site outside `validate_schema_test.go`. So the commit did not actually add startup validation for `config.yaml` despite the commit message claiming it did.
  - `P2`: the implementation is a hand-rolled subset validator, not a real draft-07 engine. Even if wired, it only handles a narrow set of schema features (`type`, `required`, `$ref`, `items`, `properties`) and ignores much of JSON Schema semantics.
- Test sufficiency:
  - Unit tests exercise the helper well.
  - Missing integration coverage: there is no config-loader/bootstrap test proving schema warnings are emitted during startup.
- Security concerns or bugs:
  - No direct security regression. The real issue is a false sense of validation coverage for operators.

## 6fa47784 `feat(health): per-model health scoring + error sanitize test coverage`

- Code quality & correctness:
  - `P1`: the commit exposes per-model internals on the public `/health` endpoint. `RunServer` registers `NewLLMModelHealthProbe` in [`internal/delivery/server/bootstrap/server.go:179`](../internal/delivery/server/bootstrap/server.go#L179), `ProviderHealth` includes `Model`, `LastError`, `FailureCount`, `ErrorRate`, and latency metrics in [`internal/infra/llm/health.go:27`](../internal/infra/llm/health.go#L27), and `HandleHealthCheck` returns component details verbatim in [`internal/delivery/server/http/api_handler_misc.go:136`](../internal/delivery/server/http/api_handler_misc.go#L136). The route itself is publicly mounted at [`internal/delivery/server/http/router.go:168`](../internal/delivery/server/http/router.go#L168), so anyone who can hit `/health` can enumerate models and see raw upstream error text.
  - `P2`: rate-limit sanitization now always claims “系统正在尝试备用模型” in [`internal/shared/errsanitize/sanitize.go:82`](../internal/shared/errsanitize/sanitize.go#L82), even when no fallback rules are configured. That is misleading in the default configuration.
- Test sufficiency:
  - The expanded sanitizer tests are good.
  - Missing tests for the new health payload, redaction policy, and route exposure. There is also no test that gates the fallback hint on actual fallback configuration.
- Security concerns or bugs:
  - Public information disclosure via `/health` is the main security issue in this set of commits.

## dc88f50d `refactor: split ExecuteTask (325→5 methods) and Prepare (411→9 methods)`

- Code quality & correctness:
  - No semantic bug stood out from the reviewed extraction. Public APIs remained stable and the split improves readability.
- Test sufficiency:
  - No new tests in the commit. For a large refactor of coordinator/preparation flow, that leaves more risk than the smaller refactors above.
- Security concerns or bugs:
  - No new security issue identified.

## acfbf95e `feat(lark): add startup phase timing and /api/health/startup-profile endpoint`

- Code quality & correctness:
  - The implementation is straightforward and internally consistent.
  - Minor note: the endpoint is added to a debug server that is already explicitly no-auth, so this does not materially worsen the threat model by itself.
- Test sufficiency:
  - Adequate for the new handler and startup-profile snapshot behavior.
- Security concerns or bugs:
  - No material new security concern beyond the existing unauthenticated debug surface.

## 87b074be `feat(llm): add provider failover on transient exhaustion (529/overloaded)`

- Code quality & correctness:
  - The retry/fallback flow is well structured and the 529 classification makes sense.
  - I did not find a concrete correctness regression in the implementation. The main design caveat is that fallback rules are keyed only by model name, not `(provider, model)`, which could become ambiguous if identical model IDs are reused across providers.
- Test sufficiency:
  - Strong unit coverage in `retry_client_test.go` for transient classification, non-fallback permanent errors, and fallback on both complete and streaming paths.
  - Missing integration coverage for config -> DI -> factory wiring of fallback rules.
- Security concerns or bugs:
  - No new security issue identified.

---

## Overall Assessment

- Highest-priority follow-ups:
  - Wire `ValidateConfigSchema` into config loading/startup and add an integration test.
  - Fix `eventDedup` to perform atomic check-and-record semantics.
  - Tie memory cleanup lifecycle to container shutdown instead of `context.Background()`.
  - Redact or remove per-model detail from the public `/health` response, or move it behind an authenticated/debug-only endpoint.
- Large refactors (`371af394`, `5207e049`, `dc88f50d`) read as mechanical, but they rely heavily on pre-existing suite health because they added no focused regression tests.
