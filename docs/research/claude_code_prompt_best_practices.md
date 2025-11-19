# Claude Code Prompt & Context Best Practices

This summary compiles guidance from Anthropic's context-engineering research and our repository's context roadmap to provide actionable, bullet-style recommendations for optimizing Claude Code prompts and the surrounding context.

## System Prompt Crafting
- **Aim for the “right altitude.”** Keep wording simple and direct so the agent has enough specificity to act without brittle, hardcoded logic or vague high-level guidance.【F:docs/research/context_engineering_article.md†L44-L53】
- **Segment prompts explicitly.** Organize instructions into named sections (e.g., `<background_information>`, `<instructions>`, `## Tool guidance`) to reduce ambiguity and make downstream maintenance safer.【F:docs/research/context_engineering_article.md†L50-L51】
- **Start minimal, then iterate.** Ship the leanest viable prompt with the strongest model, observe failure modes, and add only the instructions or examples that correct those issues.【F:docs/research/context_engineering_article.md†L52-L58】
- **Use canonical examples sparingly.** Prefer a curated set of diverse, representative few-shot examples over long edge-case lists; the goal is high-signal demonstrations instead of exhaustive rulebooks.【F:docs/research/context_engineering_article.md†L60-L61】

## Tooling & Retrieval Guidance
- **Define precise tool contracts.** Tools should be self-contained, minimally overlapping, and crystal clear about their expected usage so the agent can keep context lightweight when invoking them.【F:docs/research/context_engineering_article.md†L58-L73】
- **Favor just-in-time hydration.** Combine CLAUDE.md seeding with runtime glob/grep/file reads so the agent only pulls large artifacts when necessary, avoiding stale or bloated upfront context loads.【F:docs/research/context_engineering_article.md†L120-L134】
- **Provide cost-aware tool cues.** Surface metadata (e.g., token cost hints, preview availability) so planners can prefer token-efficient retrieval paths, as proposed in our context improvement plan.【F:docs/analysis/context_engineering_improvement_plan.md†L44-L64】

## Context Window Management
- **Treat tokens as a scarce budget.** Recognize that longer prompts cause “context rot,” so every additional token must justify its value.【F:docs/research/context_engineering_article.md†L70-L113】
- **Layer short-term vs. persistent data.** Maintain separate tracks for immediate turn history, summaries, and external notes or artifacts so you can prune aggressively without losing intent.【F:docs/analysis/context_engineering_improvement_plan.md†L18-L40】
- **Automate compaction before hard limits.** Introduce summarizers that compress stale history and strip raw tool outputs, ensuring the agent retains decisions and open tasks when the window nears capacity.【F:docs/research/context_engineering_article.md†L140-L189】【F:docs/analysis/context_engineering_improvement_plan.md†L27-L40】
- **Persist structured notes.** Encourage agents to maintain TODO/NOTES files or dedicated tools so progress survives context resets and can be reloaded intentionally.【F:docs/research/context_engineering_article.md†L189-L226】【F:docs/analysis/context_engineering_improvement_plan.md†L32-L40】

## Long-Horizon Strategies
- **Use compaction + notes + subagents.** Mix high-fidelity summaries, external notes, and specialized subagents to preserve intent during multi-hour tasks without overloading a single context window.【F:docs/research/context_engineering_article.md†L189-L260】
- **Gate large explorations behind subagents.** Allow specialized workers to spend thousands of tokens, then return concise digests (1–2k tokens) that the main loop ingests, as highlighted in Anthropic’s multi-agent research.【F:docs/research/context_engineering_article.md†L226-L260】

## Observability & Acceptance
- **Measure token economics.** Instrument cost trackers and SSE logs to capture per-turn token usage, compression triggers, and summarizer latency so regressions become visible.【F:docs/analysis/context_engineering_improvement_plan.md†L64-L90】
- **Add long-horizon acceptance tests.** Simulate 50+ turn journeys to verify that compaction, notes, and prompt modularity preserve goals while keeping token budgets under control.【F:docs/analysis/context_engineering_improvement_plan.md†L64-L115】

Adopting the practices above keeps Claude Code’s prompt stack both focused and extensible: prompts stay structured and minimal, tools stay token efficient, and context management techniques scale across long research or coding sessions.
