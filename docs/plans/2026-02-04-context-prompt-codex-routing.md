# Context Prompt: Route Coding/Planning to Codex (Prompt-only)

**Goal:** Optimize context prompt so that requests involving *software coding* or *engineering planning* are executed via Codex by default (prompt-only change; no backend routing changes).

**User request:** “涉及到编码代码或者规划相关的 都用 codex 完成” — implement by prompting the agent to delegate to Codex first.

## Approach
1. Add a policy rule in `configs/context/policies/default.yaml`:
   - When task is coding / implementation planning: attempt `bg_dispatch(agent_type=codex)` first.
   - If Codex is unavailable or fails: fall back to normal execution.
2. Verify the policy shows up in the composed system prompt (context preview / tests).
3. Run full lint + tests.
4. Commit and merge back to `main`, then clean up worktree.

## Progress Log
- 2026-02-04: Plan created.
- 2026-02-04: Updated default policy to delegate coding/planning to Codex first; added regression test verifying prompt includes `bg_dispatch(agent_type=codex)`.
- 2026-02-04: Ran `make fmt` and `make test`.
