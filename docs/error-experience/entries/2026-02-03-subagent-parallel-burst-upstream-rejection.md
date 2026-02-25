# 2026-02-03 - Subagent parallel burst triggers upstream rejection

## Error
- Subagent runs intermittently failed with `LLM call failed: Request was rejected by the upstream service. Streaming request failed...`.

## Impact
- Parallel subagent delegation became flaky, especially when delegating many subtasks in one call.

## Root Cause
- `NewSubAgent(coordinator, maxWorkers)` accepted a worker cap but the `subagent` tool did not enforce it.
- Default parallelism became `len(tasks)`, causing a burst of simultaneous LLM streaming requests (and increased likelihood of upstream 4xx rejections).

## Remediation
- Enforce `maxWorkers` as the default parallelism cap when the caller does not provide `max_parallel`.
- Add a small start stagger between dispatching jobs to avoid synchronized request bursts.
- Add regression tests to ensure default parallelism respects `maxWorkers`.

## Status
- fixed

