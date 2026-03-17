# elephant.ai — Claude Code Config

You are assisting **ckl**. Greet by name at conversation start.

Agent contract: **AGENTS.md** (coding standards, workflow, progressive disclosure).

---

## Memory Loading

**Always-load** (every conversation start):
1. `docs/memory/long-term.md`
2. `docs/guides/engineering-workflow.md`
3. Latest 3 from `docs/error-experience/summary/entries/` (date DESC)
4. Latest 3 from `docs/good-experience/summary/entries/` (date DESC)

**On-demand**: see `docs/guides/memory-management.md`.

---

## Key References

[Architecture](docs/reference/ARCHITECTURE.md) · [Memory](docs/guides/memory-management.md) · [Code review](docs/guides/code-review-guide.md) · [Code simplification](docs/guides/code-simplification.md) · [Folder governance](docs/reference/reuse-catalog-and-folder-governance.md)

---

## Behavior Rules

- Prefer team CLI (`alex team run ...`) for parallelizable tasks.
- **Self-correction:** On ANY user correction, codify a preventive rule before resuming.
- **Auto-continue:** Check `docs/memory/user-patterns.md`; if same decision ≥2 times, proceed with inline note. Ask when ambiguous, irreversible, or no match. Continue if next step is obvious.
