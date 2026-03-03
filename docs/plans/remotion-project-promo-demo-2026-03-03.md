# Remotion Project Promo Demo (2026-03-03)

## Goal
- Build a runnable Remotion demo in this repository and render one MP4 promotional clip for elephant.ai so ckl can review a concrete result.

## Scope
- Add an isolated demo project under `examples/remotion-promo/`.
- Reuse repository assets where possible (logo/image from `web/public/`).
- Produce one rendered output file under `artifacts/remotion/`.
- Keep changes local to demo/docs only; no backend/web runtime behavior changes.

## Plan
1. Scaffold Remotion app files (`package.json`, TS config, entry, root composition, scenes).
2. Implement a 20s-ish promotional timeline:
   - Intro brand scene
   - Product capabilities scene
   - Architecture scene
   - CTA/end card
3. Add render script and output target path.
4. Install dependencies and render MP4.
5. Verify output file exists and is playable metadata-wise.

## Verification
- `cd examples/remotion-promo && npm install`
- `cd examples/remotion-promo && npm run render`
- `ls -lh artifacts/remotion/elephant-promo-demo.mp4`

## Progress
- [x] Pre-work checklist on `main` completed (clean workspace, recent commits checked).
- [x] Engineering practices reviewed.
- [x] Remotion demo scaffold created.
- [x] Promo scenes implemented.
- [x] Demo video rendered and validated.

## Result
- Rendered file: `artifacts/remotion/elephant-promo-demo.mp4`
- Validation snapshot: `1920x1080`, `30fps`, `~20.05s`, `~4.1MB`, `H.264 + AAC`
