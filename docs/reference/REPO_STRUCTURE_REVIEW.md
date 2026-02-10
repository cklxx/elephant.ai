# Repo Structure Review
> Last updated: 2026-01-22

This review focuses on folder naming, top-level architecture layout, and extensibility risks. It does not propose code changes; it documents the improvement plan and priorities.

> Note: this document preserves a point-in-time review context. Some path examples
> reflect pre-refactor layout; prefer `docs/reference/ARCHITECTURE_AGENT_FLOW.md`
> for current package boundaries.

## What Is Working Well
- `cmd/` + `internal/` follow Go conventions and make the execution core discoverable.
- `web/` is clearly separated from the Go runtime and shares event semantics via the backend.
- `docs/`, `deploy/`, `scripts/`, `proto/`, and `migrations/` align with their intended roles.

## Findings (Prioritized)

### P0 — Runtime/Build Artifacts Live at the Repo Root
**Why it matters:** Root clutter hurts navigation and makes it easy to commit or reference artifacts accidentally. It also limits extensibility when adding new surfaces or build pipelines.

**Evidence**
- `make build` outputs `./alex` and `./alex-server` at the repo root (see `Makefile`).
- `logs/`, `tmp/`, and `evaluation_results/` are top-level output folders.
- Frontend artifacts appear under `web/.next`, `web/out`, and report folders.

**Recommendation**
- Standardize outputs under a single base directory such as `build/` (binaries) and `var/` (runtime artifacts).
- Update Makefile targets to emit binaries under `build/bin/` and update scripts to read from there.
- Ensure all runtime artifacts are relocatable via config or env (e.g., `ALEX_VAR_DIR`).

**Extensibility gain:** new binaries or service surfaces can be added without creating more root-level clutter.

---

### P1 — Data vs. Code Namespaces Are Mixed (Skills, Models, Artifacts)
**Why it matters:** Having both code and data at the same level increases cognitive load and creates naming collisions.

**Evidence**
- `skills/` (skill packs) exists alongside `internal/skills` (runtime code).
- `models/` (model data) sits at the root, alongside source directories.
- `evaluation_results/` is a data output sibling to `evaluation/` (source).

**Recommendation**
- Introduce a dedicated data root such as `assets/` or `data/`:
  - `assets/skills/` for skill packs.
  - `assets/models/` for local model artifacts.
  - `assets/evaluation/` or `var/evaluation/` for results.
- Keep `internal/skills` as the code namespace; avoid name collisions with data directories.

**Extensibility gain:** makes it easy to add new data domains (datasets, fixtures, model caches) without blurring source boundaries.

---

### P2 — Configuration Examples Are Split Across `configs/` and `examples/`
**Why it matters:** Developers must hunt between two locations to find canonical config vs. example config.

**Evidence**
- `configs/config.yaml`, `configs/observability.example.yaml`, and `configs/context/` live in `configs/`.
- `examples/config/runtime-config.yaml` is a primary configuration entrypoint referenced in README.

**Recommendation**
- Pick one canonical config namespace:
  - Option A: `configs/` is canonical; move example files to `configs/examples/`.
  - Option B: `examples/` is canonical for end-user recipes; move `configs/` to `internal/config/fixtures/` or `docs/`.
- Update docs to reference a single path.

**Extensibility gain:** new config surfaces (web, server, evaluator) can share a consistent example layout.

---

### P3 — Packaging/Client Surfaces Are Not Grouped
**Why it matters:** As new clients are added (mobile, IDE plugins, extra servers), a flat root grows quickly.

**Evidence**
- Go entrypoints live in `cmd/`.
- Web UI lives in `web/`.
- NPM packaging lives in `npm/`.

**Recommendation**
- Keep `cmd/` (Go convention) but consider a broader grouping for non-Go surfaces:
  - Introduce `apps/` and move `web/` to `apps/web/` if more clients are planned.
  - Move `npm/` to `packages/npm/` or `dist/npm/` to clarify it is packaging output.
- If you keep current layout, document a rule: “Go entrypoints in `cmd/`, clients in top-level folders, packaging in `dist/`”.

**Extensibility gain:** a scalable top-level taxonomy for multiple delivery surfaces.

---

### P4 — Naming Conventions Are Mixed (kebab, snake, concat)
**Why it matters:** Inconsistent naming slows navigation and complicates scripting and tooling.

**Evidence**
- `evaluation_results/` (snake_case) vs `error-experience/` (kebab-case).
- `toolregistry` (concat) vs `alex-server` (kebab).

**Recommendation**
- Adopt a naming rule:
  - Go packages: lower-case, no separators (Go import style).
  - Top-level non-Go directories: kebab-case (preferred) or snake_case, but pick one.
  - Artifacts: `build/` and `var/`.
- Normalize only when refactoring related areas to avoid churn.

**Extensibility gain:** clearer mental model and more predictable paths for new modules.

---

### P5 — Domain Vocabulary Splits (“materials” vs “attachments”)
**Why it matters:** Domain naming drift makes APIs and storage harder to reason about.

**Evidence**
- `internal/materials/` and `proto/materials/` coexist with `internal/attachments/`.

**Recommendation**
- Pick one canonical domain name (either “materials” or “attachments”).
- Add a short note in `AGENTS.md` to document the mapping until migration is complete.

**Extensibility gain:** reduces onboarding cost and avoids more synonyms as the domain grows.

---

## Suggested Target Layout (Incremental)
This is a directional layout, not an immediate refactor:

```
/
  cmd/                 # Go entrypoints
  internal/            # Go runtime packages
  apps/                # Optional: web or future clients
  assets/              # Skill packs, models, static data
  configs/             # Canonical configs + examples
  docs/                # Documentation
  deploy/              # Infra + deployment assets
  scripts/             # Automation
  proto/               # gRPC/proto definitions
  migrations/          # DB migrations
  evaluation/          # Evaluation harnesses
  build/               # Compiled binaries
  var/                 # Logs, tmp, evaluation results
```

## Execution Order (Why This Priority)
1) **P0** outputs in `build/` + `var/` (lowest risk, highest clarity).
2) **P1** data namespaces (`assets/`) to separate code from data.
3) **P2** config layout consistency.
4) **P3** client/package grouping (only if more surfaces are planned).
5) **P4–P5** naming normalization and domain vocabulary alignment.

## Non-Goals
- No immediate code or directory moves without a migration plan.
- Avoid churn that would break existing scripts or deployment paths.
