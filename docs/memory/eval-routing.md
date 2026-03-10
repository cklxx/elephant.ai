# Eval & Routing

Updated: 2026-03-10 18:00

## Eval Design

- Keep evals layered: foundation core, stateful memory, delivery quality, and frontier transfer.
- Use dedicated suites for memory capabilities, persona continuity, and speed-sensitive paths.
- Keep delivery quality separate from raw router pass rate.

## Suite Hygiene

- Retire easy saturated cases before adding more volume.
- Track `added / retired / net` each round to prevent silent dataset bloat.
- Keep reports fixed and scannable: scoreboard, top conflict clusters, sampled good and bad deliverables.
- Add hard cases by explicit benchmark dimension, not ad-hoc anecdotes.

## Routing Rules

- Add intent-level regression tests for important heuristic conflicts.
- Boost exact tool names strongly and generic tool names weakly.
- Keep Lark text-only checkpoints separate from file-delivery intents.
- For a single approved exact URL, prefer direct page retrieval over discovery flows.
