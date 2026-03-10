# User Intent Understanding Pipeline

A compiler-style pipeline that converts user input into structured intermediate states before rendering output. Every stage emits reviewable state; missing info is marked unknown, never guessed.

## Pipeline Stages

1. **Preprocess** — Segment raw text; preserve punctuation, temporal phrases, negations, pronouns.
2. **Intent skeleton** — Extract actions and goals as a function signature. Surface hidden asks (request/compare/challenge/seek plan).
3. **Slot filling** — List parameters: actor/object, scope, time window, output form, constraints, success criteria. Mark missing items as "not provided".
4. **Semantic grounding** — Resolve ambiguous terms using context (e.g., "launch" = ship vs. toggle; "faster" = latency vs. word count).
5. **Coreference recovery** — Resolve "this/that/above/as before". Keep unresolvable references as uncertain variables.
6. **Task graph** — Map to subtasks with dependencies: retrieval, reasoning, writing, calculation, code edit, comparison, advice.
7. **Consistency check** — Detect conflicting constraints, missing external info, sensitive domains. Surface minimal clarification set if unsafe.
8. **Output strategy** — Match the ask: decision → conclusions with trade-offs; learning → explanation with examples; execution → steps/commands; team comms → concise bullets.
9. **Render** — Produce natural language, explicitly stating assumptions and uncertainties.

## Correction Handling

When the user narrows scope from analysis to an immediate action (e.g., "stop service first"), execute the action first, then resume analysis.
