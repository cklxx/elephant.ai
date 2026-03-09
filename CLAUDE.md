# elephant.ai — Proactive AI Assistant

## STOP — Read this first

* You are assisting **ckl**. Every conversation opening, first greet "ckl".
* Before ANY code change on `main`, run the pre-work checklist (see `docs/guides/engineering-workflow.md` §2).
* Conflict priority: **safety > correctness > maintainability > speed**.

---

## About the user

* Seasoned backend/database engineer; fluent in Rust, Go, Python.
* Values "Slow is Fast": reasoning quality over short-term speed.
* Config files are YAML-only.

---

## Standards and workflow

All coding standards, architecture principles, testing/review process, worktree workflow, Codex protocol, and experience recording are defined in **`docs/guides/engineering-workflow.md`** — the single source of truth.

Key references:
- Architecture: `docs/reference/ARCHITECTURE.md`
- Memory: `docs/guides/memory-management.md`
- Code review: `docs/guides/code-review-guide.md`
- Code simplification: `docs/guides/code-simplification.md`
- Folder governance: `docs/reference/reuse-catalog-and-folder-governance.md`

---

## Memory loading

### Always-load set (every conversation start)
1. `docs/memory/long-term.md`
2. `docs/guides/engineering-workflow.md`
3. Latest 3 error summaries from `docs/error-experience/summary/entries/` (by filename date DESC)
4. Latest 3 good summaries from `docs/good-experience/summary/entries/` (by filename date DESC)

### On-demand loading
See `docs/guides/memory-management.md` for the full on-demand trigger table, retrieval rules, and authoring rules.

---

## Workflow preferences

* **Always use worktrees for code changes.** Never modify code directly on `main`. Use `EnterWorktree` (or `git worktree add`) to create an isolated branch, develop there, then merge back via `git merge --ff-only`.
* **Auto-merge worktree on completion.** After all commits are done in a worktree, automatically merge the branch back to `main` without asking.
* **Mark active worktrees.** Each active worktree must have `<worktree>/.worktree-active.yaml` with `status: in_progress`; never remove that worktree until status is updated to `merged` after `git merge --ff-only`.

---

## Agent behavior rules

- Prefer team CLI (`alex team run ...`) for parallelizable tasks.
- **Self-correction:** On ANY user correction, immediately codify a preventive rule before resuming.
- **Auto-continue:** Check `docs/memory/user-patterns.md` before asking; if same decision ≥2 times, proceed with inline note. Still ask when ambiguous, irreversible, or no matching pattern. At task end, continue if next step is obvious.
