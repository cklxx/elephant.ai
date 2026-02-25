# Kernel Alignment Context leaked into Lark user sessions

## Incident
- Date: 2026-02-25
- Incident ID: PM-2026-02-25-kernel-context-leak
- Severity: High
- Component(s): context assembly, preparation service, kernel alignment provider
- Reporter: ckl

## What Happened
- Symptom: Lark session first `system` message expanded to ~80k+ estimated tokens (`~83.5kt / 83.5k cum`), causing prompt bloat and noisy behavior.
- Trigger condition: `KernelAlignmentContext` was generated and injected for normal Lark runs.
- Detection channel: conversation debug page + repository code trace.

## Impact
- User-facing impact: Lark tasks started with oversized system prompt and mixed kernel-only intent with normal assistant context.
- Internal impact: token budget pressure, degraded context relevance, higher model cost and risk of truncation.
- Blast radius: all channels sharing the common preparation path where unattended guard was absent.

## Timeline (absolute dates)
1. 2026-02-12: `c3355e0f` introduced kernel alignment context into common prompt composition path.
2. 2026-02-23: `a7fcb68b` removed config gating in DI, making provider effectively always available.
3. 2026-02-25: Incident diagnosed and fixed with unattended-only injection guard + regression tests.

## Root Cause
- Technical root cause: kernel alignment provider output was injected unconditionally in preparation flow, regardless of unattended/kernel runtime context.
- Process root cause: missing boundary tests for cross-channel leakage (`normal lark` vs `unattended kernel`) after introducing shared provider.
- Why existing checks did not catch it: tests covered successful injection and provider loading, but not the negative path where injection must be absent.

## Fix
- Code/config changes:
  - Preparation now injects kernel alignment context only when `appcontext.IsUnattendedContext(ctx) == true`.
  - Added explicit regression tests for both non-unattended and unattended paths.
- Scope control / rollout strategy:
  - Minimal behavior change constrained to context assembly entry point.
  - No format/content mutation for kernel-only flows.
- Verification evidence:
  - `go test ./internal/app/agent/preparation -count=1`

## Prevention Actions
1. Add boundary tests for all runtime-only prompt injections.
   - Owner: backend/context
   - Due date: 2026-02-26
   - Validation: CI tests assert positive/negative paths.
2. Require postmortem + checklist for any cross-layer injection feature.
   - Owner: engineering process
   - Due date: 2026-02-26
   - Validation: `docs/postmortems/` template/checklist used in new incidents.
3. Introduce prompt-size observability thresholds for system message sections.
   - Owner: observability/runtime
   - Due date: 2026-03-01
   - Validation: warning when single section exceeds configured token budget.

## Follow-ups
- Open risks: kernel `GOAL.md` can grow unbounded; unattended flows still need section-level size budget.
- Deferred items: section-wise token caps for kernel alignment payload and skills metadata.

## Metadata
- id: pm-2026-02-25-kernel-context-leak
- tags: [postmortem, kernel, context, lark, token-bloat, boundary]
- links:
  - docs/error-experience/entries/2026-02-25-kernel-alignment-context-leak-into-lark-session.md
  - docs/error-experience/summary/entries/2026-02-25-kernel-alignment-context-leak-into-lark-session.md
  - docs/postmortems/checklists/incident-prevention-checklist.md
