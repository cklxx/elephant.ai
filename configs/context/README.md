# Context schema

This directory provides static context blocks loaded by internal/context/manager.go.
The loader reads all YAML files under each subdirectory and injects them into the
system prompt in a fixed order.

Subdirectories and roles:
- personas: voice and style only (identity, tone, risk profile).
- goals: objectives and success metrics.
- policies: hard constraints only (keep minimal, safety-focused). Only keep non-empty policies (currently default).
- knowledge: SOP references and memory keys.
- worlds: runtime environment facts (capabilities, limits, cost model).

Selection rules:
- Persona/world keys: ContextWindowConfig -> session metadata -> default.
- Goals/policies/knowledge: all files are loaded and injected.

Prompt assembly order:
Identity & Persona -> Mission Objectives -> Guardrails & Policies -> Knowledge & Experience
-> Skills -> Operating Environment -> Live Session State -> Meta Stewardship.

Note: when ToolMode is "web", the Operating Environment section is omitted.

Organization rules:
- Avoid duplicate instructions across layers.
- Keep persona focused on voice and guidance, not detailed checklists.
- Keep policies scoped to one concern per file and only for true guardrails.
- Keep world profiles factual and environment-specific.
