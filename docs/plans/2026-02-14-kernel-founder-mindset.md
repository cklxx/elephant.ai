# Kernel Founder Mindset Overhaul

## Context

cklxx wants the kernel to operate with a **founder's mindset** — an autonomous decision-maker that never asks questions, never waits for responses, and always drives forward. The kernel should think, plan, dispatch tasks, record state, and summarize — but never block on human input.

Current behavior: The kernel's `kernelDefaultSummaryInstruction` focuses on tool evidence and summary format but says nothing about autonomy or decision-making posture. The `DefaultSeedStateContent` is a bare scaffold. There is no system-level directive enforcing the "never ask" principle.

## Changes

### 1. Add kernel founder directive preamble (`internal/app/agent/kernel/executor.go`)

Add a new constant `kernelFounderDirective` that gets prepended to every dispatch prompt via `appendKernelSummaryInstruction` (rename to `wrapKernelPrompt` or similar):

```
你是 elephant.ai 的 kernel 自主代理，以创始人心态运作。

核心行为准则：
- 永不询问：遇到不确定性时，自主决策并记录决策理由。不要发起 request_user、clarify 等任何需要人类回应的动作。
- 永不等待：不要阻塞在任何需要外部回应的环节。如果某路径受阻，立即切换到备选方案。
- 只做四件事：思考/规划 → 派发任务（工具调用） → 记录状态 → 做总结。
- 创始人心态：对结果负全责。主动发现问题、主动解决、主动推进。不找借口、不等指令。
- 每个 cycle 必须产出可观测的进展：一个写入的文件、一次搜索结果、一个状态更新。
```

This directive is **prepended** to every agent prompt, before the agent's own task-specific prompt.

### 2. Update `kernelDefaultSummaryInstruction` (`internal/app/agent/kernel/executor.go`)

Keep the existing tool-evidence requirement but reinforce the no-ask principle:

```go
const kernelDefaultSummaryInstruction = `Kernel post-run requirement:
- You MUST complete at least one real tool action (for example: read_file, shell_exec, write_file, web_search).
- Do NOT claim completion without tool evidence.
- Do NOT use request_user, clarify, or any tool that requires human response.
- If blocked, pivot to an alternative approach. Record the blocker and your decision in the summary.
- In your final answer, include a section titled "## 执行总结".
- Summarize: completed work, concrete evidence/artifacts, decisions made, remaining risks/next step.
- Keep it concise (3-6 bullets) and factual.`
```

### 3. Refactor prompt wrapping (`internal/app/agent/kernel/executor.go`)

Rename `appendKernelSummaryInstruction` → `wrapKernelPrompt` and make it prepend the founder directive + append the summary instruction:

```go
func wrapKernelPrompt(prompt string) string {
    trimmed := strings.TrimSpace(prompt)
    if trimmed == "" {
        trimmed = kernelDefaultSummaryInstruction
    }
    var b strings.Builder
    b.WriteString(kernelFounderDirective)
    b.WriteString("\n\n")
    b.WriteString(trimmed)
    if !strings.Contains(trimmed, "## 执行总结") {
        b.WriteString("\n\n")
        b.WriteString(kernelDefaultSummaryInstruction)
    }
    return b.String()
}
```

Update the call site in `Execute()` from `appendKernelSummaryInstruction(prompt)` to `wrapKernelPrompt(prompt)`.

### 4. Update `DefaultSeedStateContent` (`internal/app/agent/kernel/config.go`)

Reflect the autonomous founder identity in the seed:

```go
const DefaultSeedStateContent = `# Kernel State
## identity
elephant.ai autonomous kernel — founder mindset.
永不询问、永不等待、只派发任务、记录状态、做总结、思考规划。
## recent_actions
(none yet)
`
```

### 5. Update tests

- `internal/app/agent/kernel/coordinator_executor_test.go`: Existing tests use `strings.Contains(runner.lastPrompt, "## 执行总结")` which remains compatible with prepending a founder directive. Add one new test `TestCoordinatorExecutor_InjectsFounderDirective` to verify the directive is present.
- No changes needed to `executor_test.go` (only tests mockExecutor, not prompt wrapping).

### 6. Update memory

Record the kernel founder mindset principle in long-term memory and MEMORY.md.

## Files to modify

1. `internal/app/agent/kernel/executor.go` — founder directive constant, refactor prompt wrapping
2. `internal/app/agent/kernel/config.go` — seed state update
3. `internal/app/agent/kernel/executor_test.go` — test updates
4. `docs/memory/long-term.md` — record principle
5. `/Users/bytedance/.claude/projects/-Users-bytedance-code-elephant-ai/memory/MEMORY.md` — record principle

## Verification

1. `go build ./...` — compiles
2. `go test ./internal/app/agent/kernel/...` — tests pass
3. `go run ./cmd/alex-server kernel-once` — single cycle executes with founder directive in prompt
4. Inspect log output to confirm prompt includes the founder preamble
