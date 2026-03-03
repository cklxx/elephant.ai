# Brand Guidelines — elephant.ai

> Living document. Last updated: 2026-03-03.

---

## Naming conventions

| Context | Name | Usage |
|---|---|---|
| **Product name** | elephant.ai | All external-facing copy, README, docs, blog, social |
| **CLI command** | `alex` | Terminal commands, code examples, developer docs |
| **Web panel** | elephant.ai Console | Web dashboard references |
| **Repo slug** | `elephant.ai` (target) | GitHub URL (currently `Alex-Code`, planned rename) |

**Rules:**
- Always lowercase `elephant.ai` in running text (except sentence-start).
- Never use "Alex" as a product name in external materials.
- `alex` (lowercase, monospace) only appears in CLI/code contexts.

---

## Color palette

| Role | Name | Hex | Usage |
|---|---|---|---|
| Primary | Indigo | `#6366f1` | CTA buttons, links, accents |
| Secondary | Purple | `#8b5cf6` | Gradients, hover states |
| Dark | Slate | `#0f172a` | Headings, nav text |
| Body text | Slate 500 | `#64748b` | Paragraph text |
| Muted | Slate 400 | `#94a3b8` | Labels, captions |
| Surface | Gray 50 | `#fafbfc` | Page background |
| Card BG | Slate 100 | `#f1f5f9` | Section backgrounds |
| Border | Slate 200 | `#e2e8f0` | Card borders, dividers |

**Gradient (primary CTA):**
```css
background: linear-gradient(135deg, #6366f1, #8b5cf6);
box-shadow: 0 4px 14px rgba(99, 102, 241, 0.25);
```

---

## Typography

| Role | Font | Weight | Fallback |
|---|---|---|---|
| Headings & body | Plus Jakarta Sans | 300–700 | system-ui, sans-serif |
| Code & mono | JetBrains Mono | 100–800 | Menlo, monospace |

Both fonts are loaded as variable WOFF2 from `web/public/fonts/`.

---

## Voice & tone

- **Confident** — state capabilities directly, no hedging ("elephant.ai does X", not "elephant.ai tries to X").
- **Warm** — approachable, first-person-plural when addressing users ("you" not "the user").
- **Efficiency-focused** — lead with outcomes and time saved, not technical implementation.
- **Non-corporate** — no buzzwords, no "leverage synergies". Write like talking to a smart colleague.
- **Bilingual** — English is primary for international reach; Chinese for domestic channels. Maintain parallel content, not translations.

**Do:**
- "Personal AI agent that lives in your workflow."
- "45+ built-in skills, triggered by a message."
- "Remembers everything. Acts autonomously."

**Don't:**
- "Enterprise-grade AI solution for workforce optimization."
- "Leveraging cutting-edge LLM technology."
- "Powered by next-gen neural architectures."

---

## Logo usage

- Primary logo: `web/public/elephant-rounded.png`
- Minimum size: 32×32px
- Always use on light/white backgrounds
- Do not stretch, rotate, or add effects
- Social preview banner: `assets/banner.png` (1280×640)

---

## Badge standards (README)

Display in this order:
1. CI status
2. Go Report Card
3. License (MIT)

Format: shield.io flat badges, linked to relevant pages.
