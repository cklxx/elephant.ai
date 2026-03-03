# GitHub Pages Home Banner + Video (2026-03-03)

## Goal
- Update the GitHub Pages home (served from `web` static export) to include a banner image and a project promo video.
- Regenerate the promo video with a visual style aligned with the homepage design language.

## Scope
- `examples/remotion-promo/`: update visual style and render output path.
- `web/public/media/`: add homepage banner/video assets.
- `web/components/home/HomeGLPage.tsx`: add a new showcase section for banner + video.

## Plan
1. Refine Remotion scene styling to align with homepage look (light background + indigo accents).
2. Render new output directly into `web/public/media/elephant-home-demo.mp4`.
3. Add/refresh banner asset under `web/public/media/home-banner.png`.
4. Add a responsive showcase section to homepage and ensure basePath-safe asset URLs.
5. Validate with web lint/build and remotion render.

## Verification
- `cd examples/remotion-promo && npm run render:web`
- `npm --prefix web run lint`
- `STATIC_EXPORT=1 NEXT_PUBLIC_BASE_PATH=/elephant.ai NEXT_PUBLIC_ASSET_PREFIX=/elephant.ai npm --prefix web run build`

## Progress
- [x] Checked deployment target: GitHub Pages publishes `web/out` (not `docs/index.html`).
- [x] Remotion style updated and video regenerated.
- [x] Homepage showcase section implemented.
- [x] Lint/build/render verification complete.

## Result
- New homepage assets:
  - `web/public/media/home-banner.png`
  - `web/public/media/elephant-home-demo.mp4`
- Homepage section added in `web/components/home/HomeGLPage.tsx` (bilingual copy, responsive banner + video).
- Validation:
  - `cd examples/remotion-promo && npx tsc --noEmit` ✅
  - `cd examples/remotion-promo && npm run render:web` ✅
  - `npm --prefix web run lint` ✅
  - `npm --prefix web run build` ✅
  - Note: direct `STATIC_EXPORT=1` build in local workspace requires stripping `web/app/api` first, which CI pages workflow already does.
