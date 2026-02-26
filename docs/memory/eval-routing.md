# Eval & Routing — Long-Term Memory Topic

Updated: 2026-02-26 15:00

Extracted from `long-term.md` to keep the main file concise.

---

## Eval Suite Design

- Foundation eval hardening works best when availability errors are explicit (`availability_error`) and heuristic routing uses action+object dual-condition gating to prevent broad-token regressions.
- Layered foundation suites (tool coverage / prompt effectiveness / proactivity / complex tasks) make routing regressions diagnosable faster than a single mixed case set.
- A dedicated memory-capabilities collection catches regressions in memory_search/memory_get and memory-informed execution chains earlier than mixed suites.
- User-habit and soul continuity routing requires separate evaluation layer; otherwise preference/persona regressions are hidden by generic memory pass rates.
- A dedicated speed-focused collection is useful to catch regressions where the router drifts to slower multi-step paths instead of shortest viable completion.
- Routing pass rates can hide delivery regressions; keep a dedicated artifact-delivery collection plus sampled good/bad deliverable checks in reports.

## Suite Growth & Maintenance

- When foundation pass@1 gets saturated, retire repeatedly top1-perfect cases and inject conflict-heavy replacements before adding more generic volume.
- Foundation eval optimization should prioritize top1 conflict clusters (`expected => top1`) with systematic router/token convergence; keep report sections fixed with x/x scoreboard, conflict inventory, and good/bad deliverable samples.
- Foundation suite growth must be budgeted with explicit caps and round-level `added / retired / net` reporting to prevent silent dataset bloat.
- After first prune under a hard threshold, a second review-driven squeeze can remove residual redundancy without losing pass@5 coverage if hard stress dimensions are kept intact.
- Foundation eval reports should keep a fixed structure: x/x scoreboard (collections/cases/pass@1/pass@5/deliverable), top conflict clusters (`expected => top1`), and sampled good/bad deliverable checks with artifact paths.
- Rebuilding evaluation should be capability-layered (Foundation Core / Stateful-Memory / Delivery / Frontier Transfer) so new hard benchmark additions expose real failed cases instead of inflating easy-pass volume.

## Heuristic & Routing

- Motivation-aware routing benefits from dedicated conflict signals (consent-sensitive boundaries, follow-up scheduling, memory-personalized cadence) and should be validated both as standalone suite and integrated full-suite regression.
- Heuristic token matching can silently miss due to stemming normalization (e.g., trailing `s` removal like `progress` -> `progres`); add intent-level regression tests for critical conflict cases instead of token-set-only assertions.
- Hard-case expansion should map to explicit benchmark-style dimensions (sparse clue retrieval / stateful commitment boundary / reproducibility trace evidence) so failures are diagnosable and optimizable as conflict families.
- Hard benchmark expansion should be taxonomy-driven (benchmark family -> capability dimension -> dataset), so future retire/add decisions remain systematic instead of ad-hoc.
- Keep suite layered by hardness (Core-Hard / Frontier-Hard / Research-Frontier-Hard); add/remove cases by layer budget to maintain challenge and diagnosability.
- Batch heuristic upgrades should apply exact-tool-name boosts asymmetrically: strong for specific tools, weak for generic tools (`plan`/`clarify`/`find`/`search_file`) to avoid cross-domain over-trigger regressions.
- For Lark delivery intents, explicitly separate "text-only checkpoint/no file transfer" (`lark_send_message`) from "must deliver file package" (`lark_upload_file`); otherwise upload dominates due shared chat/file vocabulary.
- For source-ingest intents, treat "single approved exact URL, no discovery" as strong `web_fetch` signal and suppress visual/browser capture tools unless proof/screenshot/UI language is explicit.
