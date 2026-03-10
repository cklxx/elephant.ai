# Brand Guidelines — elephant.ai

> Last updated: 2026-03-10

## Naming

| Context | Name |
|---|---|
| Product | elephant.ai (always lowercase in running text) |
| CLI command | `alex` (monospace, code contexts only) |
| Web panel | elephant.ai Console |

Never use "Alex" as a product name in external materials.

## Colors

| Role | Hex | Usage |
|---|---|---|
| Primary (Indigo) | `#6366f1` | CTA buttons, links, accents |
| Secondary (Purple) | `#8b5cf6` | Gradients, hover states |
| Dark (Slate) | `#0f172a` | Headings, nav text |
| Body text | `#64748b` | Paragraphs |
| Surface | `#fafbfc` | Page background |
| Border | `#e2e8f0` | Card borders, dividers |

Primary CTA gradient: `linear-gradient(135deg, #6366f1, #8b5cf6)`

## Typography

| Role | Font | Fallback |
|---|---|---|
| Headings & body | Plus Jakarta Sans (300–700) | system-ui, sans-serif |
| Code | JetBrains Mono (100–800) | Menlo, monospace |

Loaded as variable WOFF2 from `web/public/fonts/`.

## Voice & Tone

- **Confident** — "elephant.ai does X", not "tries to X".
- **Warm** — address users as "you", not "the user".
- **Efficiency-focused** — lead with outcomes, not implementation.
- **Non-corporate** — no buzzwords. Write like talking to a smart colleague.
- **Bilingual** — English primary, Chinese for domestic channels. Parallel content, not translations.

## Logo

- Primary: `web/public/elephant-rounded.png` (min 32x32px)
- Social banner: `assets/banner.png` (1280x640)
- Use on light backgrounds only. Do not stretch, rotate, or add effects.

## Badges (README)

Order: CI status → Go Report Card → License (MIT). Shield.io flat badges.
