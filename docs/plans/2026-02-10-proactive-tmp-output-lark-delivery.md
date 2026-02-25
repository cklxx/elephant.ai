# 2026-02-10 Proactive TMP Output + Lark Delivery Optimization Plan

## Background
User reports three gaps:
1. Agent/skills generated files are not consistently placed under `/tmp`.
2. Skill descriptions do not explicitly state runtime constraints and prerequisites (e.g. image generation backend/API requirements).
3. Proactivity is weak in Lark mode (e.g. image generated but not uploaded), requiring systematic end-to-end evaluation and optimization.

## Goals
- Enforce a clear default: temporary/generated artifacts should prefer `/tmp` unless user specifies another path.
- Make skill constraints explicit in SKILL docs, especially external backend/API/dependency limits.
- Improve prompt/context routing so Lark delivery includes proactive file upload when output files are generated for user consumption.
- Validate via end-to-end evaluation and add/update regression tests where needed.

## Scope
- Prompt/routing text in agent context/system prompts.
- Skill scripts with non-`/tmp` default output paths.
- SKILL.md docs for output-generating or dependency-constrained skills.
- E2E eval cases focused on file generation + lark delivery behavior.

## Implementation Plan
1. Baseline audit
   - Locate all file-output defaults and identify non-`/tmp` paths.
   - Locate lark upload routing guidance in prompts and execution path.
   - Locate existing eval suites/cases covering Lark delivery and proactivity.
2. TMP default alignment
   - Update remaining script defaults to `/tmp/...`.
   - Add explicit guardrails in default system prompts/context prompts for temporary output placement.
3. Proactivity upgrade (Lark file delivery)
   - Strengthen routing instructions: when produced artifact is requested/expected by Lark user, proactively upload via `lark_upload_file` after generation.
   - Keep text-only checkpoints on `lark_send_message`; avoid over-uploading.
4. Skills docs hardening
   - Update SKILL.md files with explicit prerequisites, env vars, backend availability, output defaults, and known failure modes.
5. E2E evaluation
   - Add/adjust E2E eval cases for: generated image + expected upload, file output path preference, no-upload text-only case.
   - Run targeted eval(s) and unit tests.
6. Quality gate
   - Run lint + tests (and targeted eval command outputs).
   - Mandatory code review workflow before commit.

## Risks
- Overly aggressive upload behavior could conflict with text-only intents.
- Prompt-only changes may be insufficient for all edge cases; may require routing heuristics/tests.

## Success Criteria
- Non-user-specified generated file paths default to `/tmp` in touched scripts.
- Prompt guidance explicitly distinguishes text-only vs file-delivery behavior and includes proactive upload rule.
- SKILL docs explicitly list constraints for image/video/diagram and other relevant output skills.
- E2E/targeted eval demonstrates improved behavior and no regression on text-only flows.

## Progress
- [x] Plan created
- [x] Baseline audit complete
- [x] TMP defaults aligned
- [x] Lark proactivity upgraded
- [x] Skills docs updated
- [x] E2E evaluation passed
- [ ] Review + commit + merge
