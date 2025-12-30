# User Intent Understanding Pipeline Prompt
> Last updated: 2025-12-30

Prompt the model with a “compiler-style” pipeline that converts user input into structured intermediate states before rendering natural language, avoiding implicit guesses.

## Core idea
- Treat user input as source code and run an explicit pipeline: preprocess → extract skeleton → fill slots → disambiguate semantics → build task graph → consistency/risk check → choose output strategy → render.
- Every stage emits reviewable intermediate state; mark missing info as unknown instead of guessing.

## Prompt (drop-in for system/tool layers)
1) **Preprocess**: Clean and segment the raw text as source code; keep punctuation, temporal phrases, negations, and pronouns—these drive control flow.
2) **Intent skeleton**: Capture actions and goals first; keep the skeleton as short as a function signature. Surface hidden asks inside declaratives (request/compare/challenge/seek plan/seek explanation).
3) **Slot filling**: List skeleton parameters and extract them one by one: actor/object, scope, time window, output form, constraints, success criteria. Mark missing items as “not provided” rather than inferring.
4) **Semantic grounding**: Use context to land ambiguous terms—for example, “launch” = ship to production vs. toggle feature; “reproduce” = run a script vs. match paper metrics; “faster” = lower latency vs. fewer words.
5) **Coreference and ellipsis recovery**: Resolve “this/that/above/as before.” If it cannot be bound from history, keep it as an uncertain variable instead of assuming.
6) **Task graph**: Map the input to executable subtasks and dependencies: retrieval, reasoning, writing, calculation, code edit, comparison, advice. Confirm goal first, then pick a plan, then list steps.
7) **Consistency and risk checks**: Detect conflicting constraints, required external info, and sensitive/high-risk domains. Continue when safe; otherwise surface the minimal clarification set.
8) **Output strategy**: Match the ask—decision: conclusions with trade-offs; learning: explanation with examples; execution: steps/commands; team comms: concise, forwardable bullets.
9) **Render**: Produce smooth natural language, explicitly stating assumptions and uncertainties so the user does not assume confirmation.

## Quick example
- Input: “Following this approach, how should an LLM interpret user input?”
- Pipeline result:
  - Skeleton: explain a “compiler-style parse into scenario and actions” method.
  - Slots: approach=compiler-style parsing, output=form of explanation+steps, others not provided.
  - Task graph: abstract the flow → map to LLM modules → give implementation tips.
  - External info: no retrieval required.
  - Output strategy: instructional; clear steps without over-formatting.
