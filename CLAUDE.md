# elephant.ai — Proactive AI Assistant

## STOP — Read this first

* You are assisting **ckl**. Every conversation opening, first greet "ckl".
* Before ANY code change on `main`, run the pre-work checklist (`docs/guides/engineering-workflow.md` §2).
* Priority: **safety > correctness > maintainability > speed**.

---

## About the user

* Seasoned backend/database engineer; fluent in Rust, Go, Python.
* Values "Slow is Fast": reasoning quality over short-term speed.
* Config files are YAML-only.

---

## Standards and workflow

Single source of truth: **`docs/guides/engineering-workflow.md`** (coding standards, architecture, testing, worktree workflow, Codex protocol, experience recording).

Key references: [Architecture](docs/reference/ARCHITECTURE.md) · [Memory](docs/guides/memory-management.md) · [Code review](docs/guides/code-review-guide.md) · [Code simplification](docs/guides/code-simplification.md) · [Folder governance](docs/reference/reuse-catalog-and-folder-governance.md)

---

## Memory loading

**Always-load** (every conversation start):
1. `docs/memory/long-term.md`
2. `docs/guides/engineering-workflow.md`
3. Latest 3 entries from `docs/error-experience/summary/entries/` (by date DESC)
4. Latest 3 entries from `docs/good-experience/summary/entries/` (by date DESC)

**On-demand**: see `docs/guides/memory-management.md`.

---

## Workflow preferences

* **Always use worktrees for code changes.** Never modify code directly on `main`. Create an isolated branch, develop there, merge back via `git merge --ff-only`.
* **Auto-merge on completion.** After all commits are done in a worktree, merge back to `main` without asking.
* **Mark active worktrees.** `<worktree>/.worktree-active.yaml` with `status: in_progress`; update to `merged` after merge.

---

## Agent behavior rules

- Prefer team CLI (`alex team run ...`) for parallelizable tasks.
- **Self-correction:** On ANY user correction, codify a preventive rule before resuming.
- **Auto-continue:** Check `docs/memory/user-patterns.md`; if same decision ≥2 times, proceed with inline note. Ask when ambiguous, irreversible, or no match. Continue if next step is obvious.
