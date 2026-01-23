# Plan: Diagnose Next.js build hang

## Context
- `npm run build` for `web` hangs at "Creating an optimized production build ..." with Turbopack.
- User reports the hang after recent refactor; need to isolate cause and provide mitigation.

## Steps
1. Reproduce locally and confirm whether the hang is Turbopack-specific.
2. Inspect for long-running build processes and gather build diagnostics.
3. Try disabling Turbopack and compare behavior; capture results.
4. If hang persists, identify likely bottlenecks (module graph, circular deps, large dynamic imports).
5. Document findings and provide a minimal mitigation path.

## Progress
- [x] Create plan.
- [x] Reproduce build hang locally (Turbopack stalls at "Creating an optimized production build...").
- [x] Gather diagnostics (`web/.next/diagnostics/build-diagnostics.json` shows compile stage).
- [x] Test build with webpack (`npm --prefix web run build -- --webpack`); build progresses and surfaces missing exports + type errors.
- [x] Fix missing exports + TS errors; webpack build now completes.
- [x] Summarize findings + recommended next steps.

## Summary / Next steps
- Turbopack build appears to hang during compile; webpack build completes and is a viable workaround.
- Fixed refactor regressions (barrel exports and TS type checks) that blocked webpack build.
- If Turbopack remains required, next step is to bisect module graph growth and share a minimal repro with Next.js (start with `--debug` + `--debug-build-paths` for the heaviest routes).
