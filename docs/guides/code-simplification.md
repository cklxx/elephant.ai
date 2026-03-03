# Code Simplification Best Practices

Distilled from the 2026-03-03 global codebase review ([full report](../plans/2026-03-03-global-codebase-simplify.md)). Every contributor (human and AI agent) must follow these rules.

---

## 1. Use Shared Utilities — Never Hand-Roll Primitives

**Rule:** Before writing any string/collection/HTTP helper, search `internal/shared/utils/` and neighboring shared packages first. If a utility exists, use it.

**Existing shared utilities (must-know):**

| Utility | Location | Replaces |
|---------|----------|----------|
| `utils.TrimLower(s)` | `internal/shared/utils/strings.go` | `strings.ToLower(strings.TrimSpace(s))` |
| `utils.IsBlank(s)` | `internal/shared/utils/strings.go` | `strings.TrimSpace(s) == ""` |
| `utils.HasContent(s)` | `internal/shared/utils/strings.go` | `strings.TrimSpace(s) != ""` |
| `utils.TrimDedupeStrings(ss)` | `internal/shared/utils/string_list.go` | Hand-rolled trim+dedupe+ordered loops |
| `shared.ToolError(callID, fmt, args)` | `internal/infra/tools/builtin/shared/helpers.go` | Manual `&ports.ToolResult{CallID: x, Content: err.Error(), Error: err}` |
| `httpclient.New(timeout, logger)` | `internal/infra/httpclient/httpclient.go` | Ad-hoc `&http.Client{Timeout: x}` |

**Why:** The 2026-03-03 review found ~125 hand-rolled `TrimLower`/`IsBlank`/`HasContent` calls, 50+ bypassed `ToolError()` calls, and 12+ bare `http.Client` instances (one with no timeout). These are bugs waiting to happen — the shared versions carry proxy policy, consistent error formatting, and test coverage.

**Checklist before adding a new helper:**
1. `grep -r "func.*YourFunctionName" internal/shared/` — does it already exist?
2. Check `internal/shared/utils/`, `internal/shared/markdown/`, `internal/shared/token/` for related utilities.
3. If nothing exists and the helper is used in 2+ packages, add it to the appropriate `shared/` subpackage — not as a local unexported function.

---

## 2. One Truncation Primitive — Stop Writing New Ones

**Rule:** All string truncation must go through a shared primitive. Never write a new `truncateXxx()` function.

**Background:** The review found **17 separate truncate functions** in Go and **4 in TypeScript**, all doing minor variations of "cut string at N runes, maybe add ellipsis."

**Go — use `internal/shared/utils/strings.go`:**
- For simple rune-based truncation: use the shared `Truncate` / `TruncateWithEllipsis` (if not yet added, this is CX-04 from the simplify plan; add it before writing another local variant).
- Specialized truncation (line-based, entry-count-based, metadata-aware) may remain local but must **delegate to the shared rune primitive** for the underlying cut.

**TypeScript — use `web/lib/utils.ts`:**
- `truncate()` in `web/lib/utils.ts` is the canonical version. Import it. Do not define local `truncateXxx()` functions.
- If you need different precision for `formatDuration()`, use the shared version from `@/lib/utils` with an optional parameter — never copy-paste it locally.

---

## 3. No Ad-Hoc HTTP Clients

**Rule:** Never construct `&http.Client{}` directly. Always use `httpclient.New(timeout, logger)`.

**Why:**
- The shared client applies proxy-aware transport, consistent timeouts, and transport policies.
- Ad-hoc clients bypass proxy config, may have no timeout (production incident risk), and miss observability hooks.
- The review found a client in `infra/acp/client.go` with **zero timeout** — this could hang forever on a network stall.

**Exception:** Test code may use `httptest` clients. Integration tests that need specific transport behavior should document why.

---

## 4. Error Handling — Use Types, Not Strings

**Rules:**
1. **Never classify errors by matching substrings** of error messages (`strings.Contains(err.Error(), "not found")`). Use `errors.Is()`, `errors.As()`, or typed sentinel errors.
2. **Never compare error messages** for control flow. Error text is for humans, not machines.
3. **Never extract HTTP status codes from error strings.** Carry them as structured data (e.g., `TransientError.StatusCode`).
4. **Never swallow errors silently.** If an error is logged but not returned, add a comment explaining why it's safe to ignore. If the caller needs visibility, track failure counts for monitoring.

**Anti-patterns found in the review:**
```go
// BAD: fragile string matching — "cache invalidation" contains "invalid"
if strings.Contains(err.Error(), "invalid") { return permanent }

// BAD: error message comparison — breaks if message text changes
if strings.Contains(lowerErr, strings.ToLower(sentinelErr.Error())) { ... }

// BAD: parsing HTTP status from error string
if strings.Contains(errMsg, "400") { statusCode = 400 }

// GOOD: typed error checking
var transient *errors.TransientError
if errors.As(err, &transient) { return transient.StatusCode }
```

---

## 5. Avoid God Structs — Group Fields by Responsibility

**Rule:** If a struct exceeds **12 fields**, group related fields into embedded sub-structs.

**Pattern:**
```go
// BAD: 22 flat fields
type Coordinator struct {
    llmFactory, toolRegistry, sessionStore, historyMgr,
    parser, costTracker, config, logger, clock, ...
}

// GOOD: logical grouping
type Coordinator struct {
    persistence PersistenceServices  // sessionStore, historyMgr, costTracker, checkpointStore
    execution   ExecutionServices    // llmFactory, toolRegistry, parser, externalExecutor
    proactive   ProactiveServices    // hookRegistry, timerManager, schedulerService
    config      Config
    logger      Logger
    clock       Clock
}
```

**Current offenders (fix as touched):** `RuntimeConfig` (54), `reactRuntime` (24+), `AgentCoordinator` (22), `Gateway` (25+), `TaskState` (18). See the simplify report Q-03 for suggested groupings.

---

## 6. Config Struct Discipline

**Rules:**
1. **Never create parallel config struct trees.** If you have `FooConfig` + `FooFileConfig` + `FooOverrides`, merge `FileConfig` and `Overrides` — they serve the same purpose (partial config with pointer fields for optionality).
2. **Identical config structs must be unified.** If two features have the same config shape (like `CodexConfig` and `KimiConfig`), define one struct and reuse it.
3. **Shared config types live in `shared/config`.** Never import from `internal/infra/` into `shared/config/` — that's a layer violation. Move the type definition to config if infra needs it.

---

## 7. Cache Static Computations — Never Recompute Per Iteration

**Rule:** If a value is static within a session/request, compute it once and cache it. Never recompute on every loop iteration.

**Specific patterns to avoid:**
- **JSON-serializing tool definitions** to estimate token count on every ReAct iteration. Tool definitions are static — cache the estimate.
- **Deep-cloning all messages** for read-only consumers (LLM serialization). Use copy-on-write or pass read-only references.
- **Re-normalizing already-normalized messages.** Track a watermark index; only normalize new additions.
- **Deep-cloning state for diagnostics** on every iteration. Make diagnostic recording opt-in or lazy (only clone when consumed).

**Guideline:** If you see `for` inside a function called per-iteration, ask: "Is this data changing between iterations?" If not, hoist the computation out of the loop.

---

## 8. Avoid N+1 I/O Patterns

**Rule:** Never read N individual files/records in a loop when a single batch read would suffice.

**Specific patterns to avoid:**
- Reading full file contents just to extract metadata — use a metadata index or lightweight header scan.
- Calling `GetSnapshot()` inside a loop after `ListSnapshots()` already read the same data — return full objects from the list call.
- Running N SQL queries in a loop (`CountRelated` per result) — use a single query with `WHERE path IN (...)`.
- Replaying full history to append a single new turn — accept delta-only appends.

**Guideline:** If you see a database/file read inside a `for` loop, it's almost certainly an N+1. Batch it.

---

## 9. Parallelize Independent I/O

**Rule:** When multiple I/O operations are independent (no data dependency), run them concurrently with `errgroup`.

**Common opportunities:**
- Session load + history replay + context window build during preparation
- Vector search + BM25 search in memory retrieval
- File indexing during startup (use bounded worker pool)

**Pattern:**
```go
g, gCtx := errgroup.WithContext(ctx)
g.Go(func() error { session, err = loadSession(gCtx, id); return err })
g.Go(func() error { history, err = loadHistory(gCtx, id); return err })
if err := g.Wait(); err != nil { return err }
```

---

## 10. Clean Up Long-Lived Resources

**Rule:** Every map, registry, or goroutine pool that grows over time must have an eviction or cleanup mechanism.

**Specific patterns to enforce:**
- **Background task registries:** Add TTL-based eviction. Clean up managers when sessions close or reach terminal state.
- **Event listener queues:** Have a hard maximum queue count and total timeout, not just idle timeout. Missing terminal events should not leak goroutines indefinitely.
- **Accumulated state slices** (e.g., `ToolResults`): Cap growth or retain only what's needed for downstream decisions.

---

## 11. Debounce Persistence on Hot Paths

**Rule:** Don't clone-and-persist on every single iteration. Debounce.

**Pattern:** Save at most once every N iterations or T seconds, or when state actually changed (dirty flag). The per-iteration deep-clone + async-write pattern burns CPU and GC pressure proportional to message count.

---

## 12. DRY Across Providers / Channels

**Rule:** When multiple providers or delivery channels implement the same pipeline, extract the shared skeleton.

**Patterns found:**
- LLM clients (`openai`, `anthropic`, `responses`): request-build, HTTP roundtrip, error mapping, response parse — all identical except endpoint/headers/response struct. Extract a `doCompleteRoundtrip()` helper.
- Delivery channels (Lark, Telegram): payload extraction helpers (`asString`, `asInt`, `asStringSlice`) are identical. Share them via `channels/base.go`.
- Channel configs: 15+ identical fields across `LarkChannelConfig` and `TelegramChannelConfig`. Extract `BaseChannelConfig` and embed.

---

## Quick Reference: Smell → Rule

| Code Smell | Rule |
|-----------|------|
| `strings.ToLower(strings.TrimSpace(x))` | Use `utils.TrimLower(x)` (Rule 1) |
| `strings.TrimSpace(x) == ""` | Use `utils.IsBlank(x)` (Rule 1) |
| `func truncateXxx(s string, max int)` | Use shared truncate (Rule 2) |
| `&http.Client{Timeout: x}` | Use `httpclient.New(x, logger)` (Rule 3) |
| `strings.Contains(err.Error(), "...")` | Use `errors.Is()` / `errors.As()` (Rule 4) |
| Struct with 12+ fields | Group into sub-structs (Rule 5) |
| JSON marshal in a per-iteration loop | Cache the result (Rule 7) |
| File/DB read inside `for` loop | Batch the reads (Rule 8) |
| Sequential independent I/O calls | `errgroup` (Rule 9) |
| Map/slice that only grows | Add eviction/cap (Rule 10) |
| Deep-clone + persist every iteration | Debounce (Rule 11) |
| Same logic in 2+ providers/channels | Extract shared skeleton (Rule 12) |
