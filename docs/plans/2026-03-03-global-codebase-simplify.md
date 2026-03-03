# Global Codebase Simplify — Full Report

> Date: 2026-03-03
> Scope: Historical codebase review (not recent commits)
> Status: In progress (Wave 1 + Wave 3 partial delivered on 2026-03-03)

---

## Executive Summary

Reviewed ~32K Go files and ~1.7K TypeScript files across the entire elephant.ai codebase. Found **47 findings** across three dimensions:

| Dimension | P0 | P1 | P2 | Total |
|-----------|----|----|-----|-------|
| Code Reuse | 0 | 3 | 4 | 10 |
| Code Quality | 2 | 14 | 9 | 25 |
| Efficiency | 0 | 5 | 7 | 12 |

**Top 5 highest-impact improvements:**
1. History snapshot double-read + N+1 (saves ~50% I/O per task execution)
2. Cache tool definition token estimates (eliminates hundreds of JSON marshals per task)
3. Parallelize session/history/context preparation (reduces per-task latency)
4. backgroundTaskRegistry memory leak (prevents unbounded growth in server mode)
5. 17 duplicate truncate functions → shared utility (reduces maintenance surface)

---

## Part A: Code Reuse Findings

### R-01: `TrimLower` utility exists but unused (~60 sites) ⚡ CODEX-READY

**Shared utility:** `internal/shared/utils/strings.go:6` — `utils.TrimLower()`
**Problem:** ~60 call sites hand-roll `strings.ToLower(strings.TrimSpace(x))`
**Key files:**
- `internal/infra/httpclient/validate.go` (lines 31, 35)
- `internal/infra/httpclient/proxy.go` (lines 95, 161)
- `internal/shared/config/load.go` (lines 101, 405, 471)
- `internal/shared/config/validate.go` (lines 33, 47)
- `internal/shared/config/provider_resolver.go` (lines 75, 173)
- `internal/domain/agent/react/runtime.go` (lines 362, 508, 515, 521, 1047)
- `internal/domain/agent/react/background.go` (lines 1291, 1306)
- `internal/domain/agent/react/attachments.go` (lines 203, 518, 603, 721-723)
- `internal/infra/attachments/persister.go` (lines 92, 144, 165, 184)
- `internal/delivery/server/http/` (multiple handlers)
- `internal/delivery/output/cli_renderer_helpers.go` (lines 49, 58)
- `internal/infra/coding/verify.go` (line 134)

**Action:** Replace all `strings.ToLower(strings.TrimSpace(x))` with `utils.TrimLower(x)`. Mechanical refactor.

---

### R-02: `IsBlank`/`HasContent` utilities exist but unused (~65 sites) ⚡ CODEX-READY

**Shared utility:** `internal/shared/utils/strings.go:9,12`
**Problem:** ~35 sites use `strings.TrimSpace(x) == ""`, ~30 sites use `strings.TrimSpace(x) != ""`
**Key files:**
- `internal/domain/agent/react/runtime.go` (lines 327, 625, 657, 1154)
- `internal/domain/agent/react/placeholders.go` (lines 120, 196, 293)
- `internal/domain/agent/react/attachments.go` (lines 334, 689)
- `internal/delivery/channels/lark/inject_sync.go` (lines 526, 543, 566, 585)
- `internal/app/agent/kernel/engine.go` (line 507)
- `internal/app/agent/kernel/llm_planner.go` (line 231)
- `internal/infra/llm/retry_client.go` (line 467)

**Action:** Replace all inline blank checks with `utils.IsBlank()` / `utils.HasContent()`.

---

### R-03: 17 duplicate `truncate` functions → shared utility ⚡ CODEX-READY

**Problem:** 17 separate truncate functions across the Go codebase with overlapping behavior.

| Function | File |
|----------|------|
| `truncateForLark` | `internal/delivery/channels/lark/background_progress_listener.go:734` |
| `truncateWithEllipsis` | `internal/delivery/channels/telegram/format.go:35` |
| `truncateWithEllipsis` | `internal/domain/agent/react/tool_args.go:110` |
| `truncateWithEllipsis` | `internal/delivery/output/cli_renderer_helpers.go:108` |
| `truncateStringForLog` | `internal/domain/agent/react/tool_args.go:136` |
| `truncateText` | `internal/app/agent/hooks/memory_capture.go:275` |
| `truncateSnippet` | `internal/app/context/flush_hook.go:113` |
| `truncateSkillPromptText` | `internal/app/context/manager_prompt_tools.go:256` |
| `truncateBody` | `internal/infra/llm/openai_errors.go:107` |
| `truncateEnvironmentValue` | `internal/infra/environment/utils.go:340` |
| `truncateInlinePreview` | `internal/delivery/output/cli_renderer_helpers.go:21` |
| `truncateHookText` | `internal/delivery/server/hooks_bridge.go:533` |
| `truncateToolResultContent` | `internal/domain/agent/react/tooling.go:77` |
| `truncateMemorySection` | `internal/app/context/manager_memory.go:306` |
| `truncateRecentActions` | `internal/app/agent/kernel/llm_planner.go:452` |
| `truncate` | `internal/app/context/calendar_summary.go:128` |
| `sanitizeLogValue` | `internal/infra/llm/failure_logging.go:66` |

**Action:**
1. Add `utils.Truncate(s string, maxRunes int) string` and `utils.TruncateWithEllipsis(s string, maxRunes int) string` to `internal/shared/utils/strings.go`
2. Replace all simple string-truncate functions with calls to the shared versions
3. Keep specialized ones (line-based, entry-based) but have them delegate to the primitive

---

### R-04: `TrimDedupeStrings` utility reimplemented in ~6 locations

**Shared utility:** `internal/shared/utils/string_list.go`
**Key files:**
- `internal/shared/config/load.go` (lines 232-245, 255-268)
- `internal/infra/tools/builtin/ui/options.go` (lines 16-28)
- `internal/infra/llm/anthropic_client.go` (lines 536-558)

---

### R-05: LLM `Complete()` response handling copy-pasted across 3 clients

**Files:**
- `internal/infra/llm/openai_client.go` Complete() (lines 34-198)
- `internal/infra/llm/anthropic_client.go` Complete() (lines 44-212)
- `internal/infra/llm/openai_responses_complete.go` Complete() (lines 13-141)

**Problem:** Steps 2-6 (doPost, readBody, HTTP check, unmarshal, error check) are structurally identical.
**Action:** Extract `baseClient.doCompleteRoundtrip()` helper.

---

### R-06: `shared.ToolError()` bypassed by 50+ call sites ⚡ CODEX-READY

**Shared utility:** `internal/infra/tools/builtin/shared/helpers.go:188`
**Problem:** 50+ sites construct `&ports.ToolResult{CallID: ..., Content: err.Error(), Error: err}` manually.
**Key files:**
- `internal/infra/tools/builtin/session/skills.go`
- `internal/infra/tools/builtin/web/web_search.go`
- `internal/infra/tools/builtin/aliases/read_file.go`
- `internal/infra/tools/builtin/aliases/write_file.go`
- `internal/infra/tools/builtin/larktools/upload_file.go`
- `internal/infra/tools/builtin/larktools/calendar_query.go`

---

### R-07: Duplicate payload extraction helpers between Lark and Telegram channels

**Files:**
- `internal/delivery/channels/lark/background_progress_listener.go` (line 667+)
- `internal/delivery/channels/telegram/progress_listener.go` (line 314+)

---

### R-08: 12+ ad-hoc `&http.Client{}` bypassing `httpclient.New()`

**Shared utility:** `internal/infra/httpclient/httpclient.go`
**Critical:** `internal/infra/acp/client.go:70` — creates `&http.Client{}` with **no timeout**.
**Other files:**
- `internal/infra/llamacpp/downloader.go:139`
- `internal/devops/health/checker.go:49`
- `internal/delivery/channels/lark/model_command.go:287`
- `internal/shared/config/cli_auth.go:268, 360`
- `internal/app/notification/notification.go:453`
- `internal/shared/modelregistry/registry.go:155`
- `internal/domain/materials/attachment_migrator.go:38`

---

### R-09: 5 duplicate `formatDuration()` in web frontend ⚡ CODEX-READY

**Shared:** `web/lib/utils.ts:59`
**Duplicates:**
- `web/components/agent/AgentCard/CardStats.tsx:88`
- `web/components/agent/AgentCard/CompactToolCall.tsx:90`
- `web/app/dev/conversation-debug/page.tsx:249`
- `web/app/dev/diagnostics/page.tsx:256`

---

### R-10: Frontend truncate functions fragmented (4 variants)

- `web/lib/utils.ts:136`
- `web/lib/toolPresentation.ts:97`
- `web/lib/eventAggregation.ts:152`
- Dev pages: `truncateId()`

---

## Part B: Code Quality Findings

### Q-01 [P0]: Error classification via string pattern matching

**File:** `internal/shared/errors/types.go`, lines 141-161, 215-266, 292-308, 352-416
**Problem:** `IsPermanent()` matches error strings against patterns like `"not found"`, `"invalid"`. `extractHTTPStatusCode()` is a 65-line function that parses HTTP status codes from error message strings. Matching bare `"400"`, `"401"` could match arbitrary content (port numbers, IDs).
**Fix:** Use typed error interfaces (`errors.As`) with sentinel errors. `TransientError`/`PermanentError` already carry `StatusCode int` fields. Delete `extractHTTPStatusCode()`.

---

### Q-02 [P0]: `Prepare()` — 370-line function

**File:** `internal/app/agent/preparation/service.go`, line 143
**Problem:** Handles session creation, system prompt assembly, context injection, LLM client creation, tool filtering, cost decoration, and pre-analysis in one function.
**Fix:** Decompose into `resolveSession()`, `assembleSystemPrompt()`, `createLLMClient()`, `filterTools()`, `buildTaskState()`, `runPreAnalysis()`.

---

### Q-03 [P1]: God structs (4 major instances)

| Struct | File | Fields |
|--------|------|--------|
| `RuntimeConfig` | `internal/shared/config/types.go:49` | **54** |
| `Overrides` | `internal/shared/config/types.go:545` | **50** |
| `reactRuntime` | `internal/domain/agent/react/runtime.go:20` | **24+** |
| `AgentCoordinator` | `internal/app/agent/coordinator/coordinator.go:39` | **22** |
| `ReactEngine` | `internal/domain/agent/react/engine.go:15` | **20** |
| `Gateway` (Lark) | `internal/delivery/channels/lark/gateway.go:76` | **25+** |
| `TaskState` | `internal/domain/agent/ports/agent/types.go:84` | **18** |

**Fix:** Extract logical field groups into sub-structs. E.g., for `RuntimeConfig`: `SeedreamConfig`, `ACPExecutorConfig`, `LLMCacheConfig`, `RateLimitConfig`.

---

### Q-04 [P1]: `shared/config` imports `internal/infra/tools` — layer violation

**File:** `internal/shared/config/types.go`, line 6
**Problem:** Foundation layer importing infrastructure. `ToolPolicyConfig` type defined in infra but used in config.
**Fix:** Move `ToolPolicyConfig`, `PolicyRule`, `ToolTimeoutConfig`, `ToolRetryConfig` to `shared/config`.

---

### Q-05 [P1]: Four parallel config struct hierarchies

**Files:** `types.go`, `file_config.go`, `overrides.go`
**Problem:** `RuntimeConfig` (54 fields), `RuntimeFileConfig` (~54 fields), `Overrides` (~50 fields), `applyOverrides()` (148 lines) must stay in sync. Adding a field requires touching 4 locations.
**Fix:** Use code generation or merge `RuntimeFileConfig` with `Overrides`.

---

### Q-06 [P1]: `CodexConfig` / `KimiConfig` — identical structs

**File:** `internal/shared/config/types.go`, lines 184-208
**Fix:** Define `CLIAgentConfig` and use for both.

---

### Q-07 [P1]: SQLite error detection via string matching

**File:** `internal/infra/memory/index_store.go`, lines 557-569
```go
func isMissingTable(err error) bool {
    return strings.Contains(err.Error(), "no such table")
}
```
**Fix:** Use SQLite error codes from the go-sqlite3 driver.

---

### Q-08 [P1]: Kernel notifier compares error strings

**File:** `internal/app/agent/kernel/notifier.go`, lines 166-169
```go
if strings.Contains(lowerErr, strings.ToLower(errKernelAwaitingUserConfirmation.Error())) {
```
**Fix:** Use `errors.Is()` or `errors.As()`.

---

### Q-09 [P1]: Swallowed tool call parsing error

**File:** `internal/domain/agent/react/tooling.go`, lines 61-63
**Problem:** Parse failure logged but nil returned — ReAct loop silently treats it as "no tool calls".
**Fix:** Return error to caller so ReAct loop can retry.

---

### Q-10 [P1]: `ExecuteTask()` — 352 lines

**File:** `internal/app/agent/coordinator/coordinator.go`, line 204

---

### Q-11 [P1]: `NewLarkChannel()` — 303 lines

**File:** `internal/infra/tools/builtin/larktools/channel.go`, line 72

---

### Q-12 [P1]: `StreamComplete()` — 271 lines

**File:** `internal/infra/llm/openai_client.go`, line 202

---

### Q-13 [P1]: `runtime.go` — 1234 lines

**File:** `internal/domain/agent/react/runtime.go`
**Fix:** Split into `runtime_plan.go`, `runtime_review.go`, `runtime_background.go`, `runtime_core.go`.

---

### Q-14 [P1]: Duplicate request-build logic in LLM Complete vs StreamComplete

**File:** `internal/infra/llm/openai_client.go`, lines 34-66 vs 202-235
**Fix:** Extract `buildOpenAIRequest(stream bool)` helper.

---

### Q-15 [P1]: `LarkChannelConfig` / `TelegramChannelConfig` — 15 shared fields

**File:** `internal/shared/config/file_config.go`, lines 354-429
**Fix:** Extract `BaseChannelConfig` and embed.

---

### Q-16 [P2]: Dead code — `OpenLogFile()`, `GetLogger()` have no callers

**File:** `internal/shared/utils/logger.go`, lines 62, 200

---

### Q-17 [P2]: `StreamEvent.Type` uses raw strings, not typed constants

**File:** `internal/domain/agent/ports/agent/types.go`, line 148

---

## Part C: Efficiency Findings

### E-01 [HIGH]: History snapshot double-read + N+1 ⚡ CODEX-READY

**File:** `internal/infra/session/state_store/file_store.go:114-185`
**File:** `internal/app/context/history_manager.go:126-163`
**Problem:**
1. `ListSnapshots` reads full JSON per snapshot just for metadata (N+1)
2. `HistoryManager.listSnapshots` then calls `GetSnapshot` again → every file read **twice**
**Fix:** Have `ListSnapshots` return full snapshots directly (already reads them), or add a metadata index.

---

### E-02 [HIGH]: Full history replay before append (3x I/O amplification)

**File:** `internal/app/agent/coordinator/session_manager.go:55-64`
**Problem:** `SaveSessionAfterExecution` replays entire history (`Replay(ctx, session.ID, 0)`), then `AppendTurn` calls `listSnapshots` again internally.
**Fix:** `AppendTurn` should accept delta messages only.

---

### E-03 [HIGH]: Tool definition tokens re-serialized per iteration

**File:** `internal/domain/agent/react/context_budget.go:113-130`
**Problem:** Every `think()` call JSON-serializes every tool's parameter schema and counts tokens. Tool definitions are static within a session.
**Fix:** Cache the estimate on `ReactEngine` or `Services`. Compute once.

---

### E-04 [HIGH]: Sequential session/history/context preparation

**File:** `internal/app/agent/preparation/service.go:143-262`
**Problem:** `loadSession`, `loadSessionHistory`, `contextMgr.BuildWindow` are independent but run sequentially.
**Fix:** Use `errgroup` to parallelize.

---

### E-05 [HIGH]: `backgroundTaskRegistry` never evicts completed managers (memory leak)

**File:** `internal/app/agent/coordinator/background_registry.go:12-46`
**Problem:** `map[string]*BackgroundTaskManager` grows indefinitely. Each manager has goroutines/channels.
**Fix:** Add TTL-based eviction or cleanup on session close.

---

### E-06 [MEDIUM]: Sequential vector + BM25 search

**File:** `internal/infra/memory/indexer.go:183-191`
**Fix:** Run both searches in parallel via `errgroup`.

---

### E-07 [MEDIUM]: `normalizeContextMessages` runs on every think()

**File:** `internal/domain/agent/react/solve.go:46`
**Problem:** Idempotent normalization re-runs on all messages every iteration.
**Fix:** Track `normalizedUpTo` index; only normalize new messages.

---

### E-08 [MEDIUM]: Deep-clone all messages per think()

**File:** `internal/domain/agent/react/messages.go:7-23`
**Problem:** Full deep clone of all messages every iteration. Messages used read-only by LLM.
**Fix:** Copy-on-write or avoid cloning read-only messages.

---

### E-09 [MEDIUM]: Deep-clone state for diagnostics per iteration

**File:** `internal/domain/agent/react/context.go:46-66`
**Fix:** Make turn recording opt-in or lazy.

---

### E-10 [MEDIUM]: Session clone per iteration for async save

**File:** `internal/app/agent/coordinator/session_manager.go:136-151`
**Fix:** Debounce async save (at most once every 3 iterations or 5 seconds).

---

### E-11 [MEDIUM]: `state.ToolResults` grows without bounds

**File:** `internal/domain/agent/react/runtime.go:925`
**Fix:** Only keep last N results, or track a boolean.

---

### E-12 [MEDIUM]: Cost store reads all records, filters in Go

**File:** `internal/infra/storage/cost_store.go:85-121`

---

## Codex Execution Plan

### Wave 1: Mechanical Refactors (safe, parallel) ⚡

These are search-and-replace operations that Codex can execute autonomously:

| Task ID | Finding | Estimated Files | Verify Command |
|---------|---------|----------------|----------------|
| CX-01 | R-01: Replace `strings.ToLower(strings.TrimSpace(x))` → `utils.TrimLower(x)` | ~30 | `go build ./...` |
| CX-02 | R-02: Replace `strings.TrimSpace(x) == ""` → `utils.IsBlank(x)` | ~20 | `go build ./...` |
| CX-03 | R-02b: Replace `strings.TrimSpace(x) != ""` → `utils.HasContent(x)` | ~20 | `go build ./...` |
| CX-04 | R-03: Add `utils.Truncate()` + `utils.TruncateWithEllipsis()` to strings.go | 1 | `go test ./internal/shared/utils/...` |
| CX-05 | R-09: Delete duplicate `formatDuration()` in web, import from `@/lib/utils` | 4 | `cd web && npx vitest run` |
| CX-06 | Q-16: Delete dead code `OpenLogFile()`, `GetLogger()` | 1 | `go build ./...` |

Execution (2026-03-03):
- [x] CX-01 completed (target ownership set under `internal/` migrated to `utils.TrimLower`).
- [x] CX-02 completed (target ownership set under `internal/` migrated to `utils.IsBlank`).
- [x] CX-03 completed (target ownership set under `internal/` migrated to `utils.HasContent`).
- [~] CX-04 partial: shared `utils.Truncate()` / `utils.TruncateWithEllipsis()` added, and first call-site batch migrated (`memory_capture`, `flush_hook`, `manager_memory`).
- [x] CX-05 completed (web duplicate `formatDuration` call sites consolidated to `@/lib/utils` with local behavior-preserving wrappers).
- [~] CX-06 partial: `GetLogger()` removed; `OpenLogFile()` retained because still used by `cmd/alex/container.go`.

### Wave 2: Targeted Quality Fixes (moderate complexity)

| Task ID | Finding | Estimated Files | Verify Command |
|---------|---------|----------------|----------------|
| CX-07 | Q-06: Merge `CodexConfig`/`KimiConfig` → `CLIAgentConfig` | 3-5 | `go build ./...` |
| CX-08 | Q-08: Replace error string comparison in notifier with `errors.Is()` | 1 | `go test ./internal/app/agent/kernel/...` |
| CX-09 | Q-14: Extract `buildOpenAIRequest()` helper | 1 | `go test ./internal/infra/llm/...` |
| CX-10 | Q-15: Extract `BaseChannelConfig` for shared channel fields | 2-3 | `go build ./...` |
| CX-11 | R-06: Replace manual `ToolResult` construction → `shared.ToolError()` | ~15 | `go build ./...` |
| CX-12 | R-08: Replace ad-hoc `&http.Client{}` → `httpclient.New()` | ~10 | `go build ./...` |

Execution (2026-03-03, rescan continuation):
- [ ] CX-07 pending.
- [x] CX-08 completed: notifier now prioritizes structured `FailureClass` over error-string substring matching; fallback keeps compatibility when class is missing.
- [ ] CX-09 pending.
- [ ] CX-10 pending.
- [~] CX-11 partial: first batch (`aliases/read_file`, `aliases/write_file`, `session/skills`) migrated to `shared.ToolError()`.
- [~] CX-12 partial: first non-web batch migrated to `httpclient.New()` (`infra/acp/client`, `infra/llamacpp/downloader`, `devops/health/checker`) with behavior-preserving timeout/transport handling.

Execution (2026-03-03, rescan wave C — non-web):
- [~] CX-07 in progress (design/scan completed, deferred execution in wave C).
- [x] CX-08 completed in previous wave.
- [x] CX-09 completed: extracted shared `buildOpenAIRequest(req, stream bool)` in `openai_client.go`; parity tests added.
- [ ] CX-10 pending.
- [~] CX-11 partial: additional R-06 slice migrated (`aliases/read_file`, `aliases/write_file`, `aliases/shell_exec`, `aliases/replace_in_file`, `larktools/upload_file`, `larktools/calendar_query`).
- [~] CX-12 partial: additional R-08 slice migrated (`infra/llamacpp/downloader`, `shared/config/cli_auth`, `shared/modelregistry/registry`, `delivery/channels/lark/model_command`, `app/notification/notification`).

Execution (2026-03-03, rescan wave D — non-web):
- [x] CX-07 completed (alias-first): introduced `CLIAgentConfig` + `CLIAgentFileConfig`; `Codex*`/`Kimi*` converted to aliases.
- [x] CX-08 completed in previous waves.
- [x] CX-09 completed in previous wave.
- [ ] CX-10 pending (deferred; medium risk around YAML flattening + loader coupling).
- [~] CX-11 partial: additional R-06 batch migrated (`session/skills`, `orchestration/reply_agent`, `larktools/{bitable_manage,calendar_create,calendar_update,channel,contact_manage,docx_manage,drive_manage,mail_manage,okr_manage,sheets_manage,vc_manage,wiki_manage}`); remaining concentrated in `task_manage` + `lark_oauth` + helper internals.
- [~] CX-12 partial: additional R-08 batch migrated (`cmd/alex/{cli_model,cli_model_picker,lark_scenario_cmd}`, `evaluation/swe_bench/dataset`, `internal/devops/health/checker`); remaining non-test ad-hoc clients are `infra/acp/client` and `domain/materials/attachment_migrator`.

Execution (2026-03-03, rescan wave E/F — all-in):
- [x] CX-07 completed in previous wave D.
- [x] CX-08 completed in previous waves.
- [x] CX-09 completed in previous wave.
- [x] CX-10 completed: introduced `BaseChannelConfig` and shared channel env-expansion helper in `file_loader`; kept YAML compatibility via inline embedding.
- [x] CX-11 completed (scope-closed): migrated remaining practical call-sites (`web_search` fallback, `larktools/channel`, `larktools/task_manage`, `larktools/lark_oauth`, `session/skills` load-path). Residual manual constructors are intentional utility internals (`shared/helpers`) and one custom-content branch in `session/skills`.
- [x] CX-12 completed: migrated remaining non-test ad-hoc clients (`infra/acp/client`, `domain/materials/attachment_migrator`) to `httpclient.New(...)`.

Execution (2026-03-03, rescan wave G — non-web):
- [~] CX-01 continuation: migrated additional non-web `TrimLower` batches in `cmd/alex`, selected `internal/` hot paths (`react` + `attachments`), and `evaluation/agent_eval` + `ports/mocks` from `strings.ToLower(strings.TrimSpace(...))` to `utils.TrimLower(...)`.
- [~] CX-02/CX-03 continuation (evaluation scope): migrated `strings.TrimSpace(...) ==/!= ""` checks in `evaluation/agent_eval` to `utils.IsBlank/HasContent`.
- [~] CX-02/CX-03 continuation (runtime non-web scope): migrated selected blank/non-blank checks in `internal/app/agent/*`, `internal/delivery/channels/lark`, `internal/domain/agent/react`, `internal/infra/llm`, `internal/devops/process`, `internal/infra/external/teamrun`, and `internal/domain/agent/ports`.
- [~] CX-01/CX-02/CX-03 residual cleanup: completed remaining non-web mechanical sites in `internal/{domain/agent/taskfile,devops,delivery/server/bootstrap,infra/tools/builtin/artifacts,shared/config}` and `scripts/memory/backfill_networked.go` (excluding helper definitions themselves).
- [x] Validation for this batch passed (`go test` + targeted golangci-lint on touched packages).

### Wave 3: Efficiency Improvements (requires careful design — plan first)

| Task ID | Finding | Risk | Approach |
|---------|---------|------|----------|
| CX-13 | E-03: Cache tool token estimates | Low | Add `cachedToolTokens int` field on engine |
| CX-14 | E-05: backgroundTaskRegistry eviction | Medium | Add TTL-based cleanup goroutine |
| CX-15 | E-06: Parallel vector + BM25 search | Low | `errgroup.Go()` wrapper |
| CX-16 | E-01/E-02: History snapshot I/O optimization | High | Requires design — skip for now |
| CX-17 | E-04: Parallel preparation | Medium | Requires understanding of data dependencies |

Execution (2026-03-03):
- [x] CX-13 completed: tool definition token estimate now cached on `ReactEngine` with thread-safe invalidation by tool signature.
- [x] CX-14 completed: `backgroundTaskRegistry` now has TTL+terminal-state cleanup with tests.
- [x] CX-15 completed: memory vector/BM25 search now runs in parallel with cancellation-aware error convergence and tests.
- [ ] CX-16 deferred (high-risk, requires dedicated design pass).
- [ ] CX-17 deferred (needs dependency analysis in preparation flow).

Execution (2026-03-03, rescan wave E/F):
- [x] CX-16 completed: added `SnapshotPayloadLister` fast-path and switched history replay loading to payload pages (eliminates List+Get N+1 pattern for supported stores); fixed file-store cursor pagination semantics and added payload pagination tests.
- [x] CX-17 completed: preparation critical path now runs context-window build and LLM client initialization concurrently with cancellation-aware coordination; behavior parity kept for prompt/system-state assembly.

### Wave 3b: Low-risk efficiency continuation (2026-03-03, rescan)

| Task ID | Finding | Result |
|---------|---------|--------|
| CX-18 | E-12: Cost store reads all records, filters in Go | [x] Completed: `GetByModel()` now scans date dirs directly and filters while reading (no `ListAll()` full replay); ordering preserved by shared timestamp sort helper; tests updated. |

## Validation Snapshot (2026-03-03)

- `go test ./...` passed.
- `go test -race -count=1 ./...` hit one intermittent failure at `internal/domain/agent/react TestMultipleTasks` (`expected 5 completions, got 4`); targeted rerun `go test -race -count=1 ./internal/domain/agent/react -run TestMultipleTasks` passed.
- `./scripts/run-golangci-lint.sh run --timeout=10m ./...` passed.
- `npm --prefix web run lint` passed.
- `npm --prefix web run build` passed.
- `./scripts/pre-push.sh` still flaky in this environment on two checks:
  - `go test -race` intermittent failure (passes on standalone rerun).
  - `golangci-lint` 3-minute timeout in script (passes with 10-minute timeout).

Validation Snapshot (2026-03-03, wave E/F):

- `go test ./...` passed.
- `go test -race -count=1 ./...` passed.
- `./scripts/run-golangci-lint.sh run --timeout=10m ./...` passed.
- `npm --prefix web run lint` passed (after `npm --prefix web install` in local worktree where `eslint` was initially missing).
- `npm --prefix web run build` passed.

### Wave 4: Architectural (requires design review — not Codex-ready)

| Finding | Reason |
|---------|--------|
| Q-01: Error classification system | Needs error type hierarchy redesign |
| Q-02/Q-10: Long function decomposition | Needs understanding of logical boundaries |
| Q-03: God struct decomposition | Needs sub-struct design review |
| Q-04/Q-05: Config layer violation + parallel hierarchies | Needs architectural discussion |
| Q-13: runtime.go file split | Needs file boundary design |
