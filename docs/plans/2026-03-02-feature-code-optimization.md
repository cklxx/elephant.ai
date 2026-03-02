# Feature Code Optimization Plan

**Date**: 2026-03-02
**Scope**: Recent feature code review across 5 modules
**Principle**: Maximum simplicity and elegance — delete > refactor > add

---

## Executive Summary

Review of recent feature commits found **48 issues** across 5 modules. The biggest win is deleting ~2500 lines of dead/broken code (evolution + kernel fallback), then eliminating cross-package duplication in context optimization, and fixing a P0 bug in Lark message editing.

| Module | P0 | P1 | P2 | P3 | Action |
|--------|----|----|----|----|--------|
| Evolution | 6 | 10 | 6 | — | **Delete entire package** |
| Kernel Fallback | 2 | 4 | 3 | — | **Delete files** |
| Taskfile | — | 5 | 5 | 3 | Refactor |
| Lark Post | 1 | 5 | 6 | — | Fix P0 + simplify |
| Context Optimization | — | 6 | 7 | 2 | Deduplicate |

---

## Phase 1: Delete Dead Code (estimated: ~2500 LOC removed)

### 1.1 Delete `internal/domain/agent/evolution/` entirely

**Why**: All 9 files carry `//go:build ignore`. The code doesn't compile — 15+ undefined identifiers in `engine.go`, incompatible interface vs struct designs, struct field mismatches everywhere. This is a broken skeleton that can never be incrementally fixed.

**Files to delete**:
- `internal/domain/agent/evolution/agent.go`
- `internal/domain/agent/evolution/analyzer.go`
- `internal/domain/agent/evolution/config.go`
- `internal/domain/agent/evolution/engine.go`
- `internal/domain/agent/evolution/feedback.go`
- `internal/domain/agent/evolution/memory.go`
- `internal/domain/agent/evolution/optimizer.go`
- `internal/domain/agent/evolution/provider.go`
- `internal/domain/agent/evolution/types.go`
- `configs/evolution_example.yaml`

### 1.2 Delete kernel fallback config (dead code, never wired in)

**Why**: `LoadFallbackConfig`, `GetFallbackForAgent`, `ShouldTriggerFallback` have zero callers. The recovery status file explicitly says "blocked_by: Requires code change to consume". Duplicate `CircuitBreakerConfig` struct conflicts with the real one in `internal/shared/errors/`. Hand-rolled `containsIgnoreCase` is wrong for non-ASCII.

**Files to delete**:
- `internal/app/agent/kernel/fallback_config.go`
- `configs/llm_provider_fallback.yaml`

---

## Phase 2: Fix P0 Bug

### 2.1 Lark progress-message edit discards post format

**File**: `internal/delivery/channels/lark/task_manager.go:617`

**Bug**: `smartContent()` computes `replyMsgType`/`replyContent` correctly (detecting post format), but the edit path calls `g.updateMessage(progressMsgID, reply)` which hardcodes `"text"` format, bypassing the post conversion. Markdown-heavy edits are sent as raw text.

**Fix**: Pass `msgType` through `updateMessage`, or use `replyMsgType`/`replyContent` in the edit path:
```go
// gateway.go — add msgType parameter
func (g *Gateway) updateMessage(ctx context.Context, messageID, msgType, content string) error {
    return g.messenger.UpdateMessage(ctx, messageID, msgType, content)
}

// task_manager.go — use smartContent result
if err := g.updateMessage(execCtx, progressMsgID, replyMsgType, replyContent); err != nil {
```

---

## Phase 3: Taskfile Module Cleanup

### 3.1 Fix dead logic in `AnalyzeMode` — `mode.go:54-58`

The final `if` block and trailing `return ModeSwarm` both return ModeSwarm — rule 3 is not implemented. Either implement the intended third outcome or collapse to two rules.

### 3.2 Lift `Validate` call to `Executor.Execute` — single validation point

Currently `Validate` is called independently in both `executeTeam` and `ExecuteSwarm`, each of which also calls topo sort again. Validate once before the mode branch.

### 3.3 Eliminate `ExecuteAndWait` swarm duplication — `executor.go:59-63`

Swarm branch in `ExecuteAndWait` is identical to `Execute`. Delegate:
```go
if e.resolveMode(tf) == ModeSwarm {
    return e.Execute(ctx, tf, causationID, statusPath)
}
```

### 3.4 Fix `newOrderTracker` unused parameter — `swarm_test.go:24-28`

`resultStatus` is accepted but never used. Either wire it in or remove it.

### 3.5 Delete dead test code — `swarm_test.go:304-305`

```go
origDispatch := mock.Dispatch
_ = origDispatch
```

### 3.6 Add `SwarmConfig` invariant: `InitialConcurrency <= MaxConcurrency`

### 3.7 Fix hardcoded `/tmp` path in test — `swarm_test.go:272`

Use `filepath.Join(t.TempDir(), ...)` like all other tests.

---

## Phase 4: Lark Module Simplification

### 4.1 Fix italic detection/conversion mismatch — `markdown_to_post.go`

Italic is detected in `hasMarkdownPatterns` but never converted in `convertInlineMarkdown`. Either add conversion or remove detection.

### 4.2 Deduplicate `rephrase.go` LLM call with `narrateWithLLM`

`rephraseForUser` duplicates the same client/timeout/complete/trim pattern. Delegate to `narrateWithLLM`.

### 4.3 Fix bold stripping — `markdown_to_post.go:194`

Replace `strings.Trim(raw, "*")` with `raw[2:len(raw)-2]` for correctness.

### 4.4 Eliminate text-format duplication between `smartContent` and `textContent`

`smartContent`'s text fallback manually inlines `textContent`. Use `textContent` directly.

### 4.5 Inline `postBody` single-field wrapper — `markdown_to_post.go:22-24`

Replace with anonymous struct in `postPayload`.

---

## Phase 5: Context Optimization Deduplication

### 5.1 Fix `toolMentions` double-count — `manager_compress.go:213`

Move `toolMentions += len(msg.ToolResults)` inside the `case "tool":` switch branch.

### 5.2 Extract shared `keepRecentTurns` — eliminate duplication

`manager_compress.go:379` and `solve.go:640` are identical. Extract to a shared utility in `ports` or a context util package.

### 5.3 Extract shared `isPreservedSource` predicate

`manager_compress.go:367` and `context_checkpoint.go:202` test the same set of sources. Move to `ports` where `MessageSource` is defined.

### 5.4 Deduplicate `deriveRepoRoot` — `manager.go:165` vs `static_registry.go:338`

One should call the other.

### 5.5 Delete `rebuildStateMessages` — `solve.go:665-672`

Function ignores its `original` parameter and just returns `trimmed`. Replace callsites with direct assignment.

### 5.6 Remove `_ = strings.TrimSpace(provider)` stub — `context_overflow_classifier.go:17`

Either implement provider-specific logic or remove the parameter.

### 5.7 Extract named constants for magic numbers — `solve.go:548-621`

`24`, `64`, `512`, `256`, `12`, `32` — all need named constants with rationale comments.

### 5.8 Extract shared `truncateToRunes` helper

`buildCompressionSnippet` and `truncateSkillInlineText` are structurally identical.

### 5.9 Deduplicate compression-summary prefix strings

`manager_window.go` and `context_artifact_compaction.go` define the same 3 prefix strings under different names. Extract to `ports`.

### 5.10 Avoid `NewFeedbackStore` per-turn allocation — `manager_prompt_tools.go:108-112`

Move store creation to manager initialization, pass as field.

---

## Execution Order

```
Phase 1 (Delete) → Phase 2 (P0 Fix) → Phase 3 (Taskfile) → Phase 4 (Lark) → Phase 5 (Context)
```

Each phase is independently committable. Phase 1 has zero risk (deleting dead code). Phase 2 fixes a user-visible bug. Phases 3-5 are structural improvements.

---

## Out of Scope (tracked for future)

- `TaskSpec` god-struct refactoring (P3) — extract `CodingConfig` sub-struct
- Ordered list handling in `buildPostContent` (P2)
- `contextOverflowClassification.Confidence` dead field cleanup (P2)
- Resolve `warningThreshold` / `defaultThreshold` latent coupling (P2)
