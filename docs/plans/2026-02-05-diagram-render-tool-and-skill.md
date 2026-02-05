# Plan: diagram_render tool + diagram-to-image skill

Date: 2026-02-05
Owner: cklxx + Codex
Branch: `eli/diagram-render` (worktree)

## Goal
- Add a built-in tool `diagram_render` that **offline-renders** Mermaid and icon blocks into **beautiful PNG** (optionally SVG), suitable for auto-upload to Lark.
- Add a skill `diagram-to-image` that guides usage + auto-triggers.

## Status
- [x] Worktree created; `.env` copied
- [ ] Tool implementation (`internal/tools/builtin/diagram/`)
- [ ] Tool registered (`internal/toolregistry/registry.go`)
- [ ] Unit tests
- [ ] Skill (`skills/diagram-to-image/SKILL.md`)
- [ ] Full lint + tests
- [ ] Merge back to `main` + cleanup worktree

## Implementation outline (decision complete)
1) Embed Mermaid JS offline (`internal/tools/builtin/diagram/assets/mermaid.min.js`) + asset README.
2) Implement `diagram_render`:
   - `format=mermaid`: normalize source (strip ```mermaid fences), render via embedded mermaid.js in HTML, wait for `data-diagram-status=ready`, screenshot `#capture`, optionally emit SVG.
   - `format=icon_blocks`: render HTML cards grid; screenshot `#capture`.
   - Attachments: `<name>.png` and optional `<name>.svg`.
3) Provide **local** and **sandbox** executors:
   - local: `chromedp.NewExecAllocator(...)`
   - sandbox: fetch CDP URL via `/v1/browser/info`, then `chromedp.NewRemoteAllocator(...)`
4) Register tool for both toolsets in `internal/toolregistry/registry.go`.
5) Add skill with triggers and YAML examples.
6) Add unit tests (HTML generation + normalization + naming).

## Notes
- Defaults: `theme=light`, `output=png`, `width=1200`, `height=800`, `scale=1.0`, `padding=32`.
- Avoid data URLs for HTML; use `page.SetDocumentContent`.
- Prefer element screenshot (`chromedp.Screenshot("#capture", ...)`) for tight crop.

