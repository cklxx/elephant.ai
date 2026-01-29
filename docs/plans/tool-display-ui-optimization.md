# Plan: Tool Display UI Optimization

## Status: Draft
## Date: 2026-01-29

---

## Problem Analysis

Current tool expand/collapse UI has several issues:

### Collapsed State (Header)
1. **No expand/collapse indicator** — no chevron icon, users can't tell it's clickable
2. **Too compact padding** — `px-1 py-0.5` in ToolOutputCard is cramped
3. **No summary text** in ToolOutputCard (ToolCallCard has it, but ToolOutputCard doesn't)
4. **Duration not shown** in ToolOutputCard header (computed but unused)
5. **Status badge computed but never rendered** (`statusBadgeVariant` exists but isn't used)
6. **Washed-out appearance** — `bg-secondary/40 border-border/40` is too faint

### Expanded Content
7. **No expand/collapse animation** — instant show/hide causes layout jumps
8. **Double-border card-in-card** — SimplePanel's `rounded-xl border` inside the outer card is visually noisy
9. **Forced 2-column grid** — `lg:grid-cols-2` doesn't adapt when there's only 1 panel
10. **ALL_CAPS panel headers** — `uppercase tracking-wide` looks dated/aggressive
11. **Fixed `max-h-64` height** — too short for meaningful code or output
12. **No visual hierarchy** between parameters, result, and metadata panels

### General
13. **Inconsistent font sizes** — 10px, 11px, 12px, 13px all used without clear system
14. **Tool icon area too small** — `h-4 w-4` feels cramped
15. **No framer-motion dependency** — pure CSS transitions only (limits animation quality)

---

## Industry Reference (Top Patterns)

| Product | Key Pattern | Why It Works |
|---------|------------|--------------|
| **ChatGPT** | Collapsible accordion with "Searched 5 sites" summary | Progressive disclosure; minimal default |
| **Claude** | Artifacts in side panel with Preview/Code toggle | Separates output from conversation |
| **Cursor** | Inline file pills + colored diffs | Context-aware compact display |
| **v0/Vercel** | Generative UI — tool results as custom React components | Rich, tool-specific rendering |
| **Devin** | Confidence dots (green/yellow/red) + temporal scrubbing | Status at a glance |
| **Perplexity** | Inline citations + source cards at top | Woven into prose, not separate |

**5 Qualities of Polished Tool UI:**
1. Progressive disclosure (compact summary → full detail)
2. Visual hierarchy separation (tool output ≠ assistant text)
3. Smooth animation (200-300ms ease-out height transition)
4. Contextual loading text ("Searching the web..." not generic spinner)
5. Consistent 3-state display (running/success/error)

---

## Proposed Changes

### Phase 1: Header Polish (ToolCallCard + ToolOutputCard)

#### 1.1 Add chevron indicator with rotation animation
- Add `ChevronRight` from lucide-react to header
- Rotate 90° on expand with `transition-transform duration-200`
- Place between status icon and tool name

#### 1.2 Increase header padding and improve layout
- ToolOutputCard: `px-1 py-0.5` → `px-3 py-1.5` (match ToolCallCard)
- Both: add `gap-x-3` consistent spacing
- ToolOutputCard: adopt grid layout `grid grid-cols-[16px,auto,1fr,auto]`

#### 1.3 Show duration in ToolOutputCard header
- Already computed (`displayDurationMs`) but not rendered
- Add duration display matching ToolCallCard pattern

#### 1.4 Strengthen visual presence
- Both: `bg-secondary/40 border-border/40` → `bg-muted/50 border-border/60`
- Running: keep blue theme but slightly more saturated
- Error: keep red theme
- Done: slightly more visible than current wash-out

#### 1.5 Show summary text in ToolOutputCard
- Add one-line result summary below tool name (same as ToolCallCard)
- Use `userFacingToolSummary()` which already exists

**Files:** `ToolOutputCard.tsx`, `ToolCallCard.tsx`

### Phase 2: Expand/Collapse Animation

#### 2.1 CSS-only height animation using grid hack
Since framer-motion is not available and we want zero new dependencies:
```css
/* Container with CSS grid transition */
.tool-expand-container {
  display: grid;
  grid-template-rows: 0fr;
  transition: grid-template-rows 200ms ease-out;
}
.tool-expand-container[data-expanded="true"] {
  grid-template-rows: 1fr;
}
.tool-expand-inner {
  overflow: hidden;
}
```
This is the modern CSS approach — no JS height measurement, smooth animation, no layout jump.

#### 2.2 Content fade-in
- Add `opacity` transition: 0 → 1 over 150ms (delayed 50ms after height starts)
- Use Tailwind: `transition-opacity duration-150 delay-50`

**Files:** `ToolOutputCard.tsx`, `ToolCallCard.tsx`, possibly `globals.css` for the grid-row transition utility class

### Phase 3: Expanded Content Cleanup

#### 3.1 Remove double-border (SimplePanel redesign)
- SimplePanel current: `rounded-xl border border-border bg-card/90 p-4`
- Change to: remove border, remove bg, just spacing and layout
- The outer card already provides visual containment
- SimplePanel becomes a layout container, not a card

#### 3.2 Adaptive column layout
- Replace `lg:grid-cols-2` with auto-fit: `grid-cols-[repeat(auto-fit,minmax(300px,1fr))]`
- Single panel → full width, two+ panels → columns

#### 3.3 Panel header modernization
- Remove `uppercase tracking-wide` — use sentence case, medium weight
- `text-[11px] font-semibold uppercase` → `text-xs font-medium text-muted-foreground`
- Add subtle left border accent for visual grouping: `border-l-2 border-primary/20 pl-3`

#### 3.4 Increase code block height limit
- `max-h-64` (256px) → `max-h-80` (320px) for result panels
- Arguments panel: keep `max-h-64` (usually shorter)

#### 3.5 Font size normalization
- Header tool name: 13px (keep)
- Header summary: 12px (keep)
- Panel title: 12px (was 11px)
- Panel content: 12px mono (keep)
- Duration/meta: 11px (keep)

**Files:** `ToolPanels.tsx`, `ToolOutputCard.tsx`, `ToolCallCard.tsx`

### Phase 4: Status & Running State Enhancement

#### 4.1 Improve running state header
- Add subtle pulse animation to running icon (already has `animate-spin` on Loader2)
- Add pulsing left border accent: `border-l-2 border-blue-400 animate-pulse`

#### 4.2 Error state improvement
- Show error summary inline in collapsed header (red text, truncated)
- Expanded: error panel with red left border accent

**Files:** `ToolOutputCard.tsx`, `ToolCallCard.tsx`

---

## Files to Modify

| File | Changes |
|------|---------|
| `web/components/agent/ToolOutputCard.tsx` | Header: chevron, padding, duration, summary, animation wrapper |
| `web/components/agent/ToolCallCard.tsx` | Header: chevron, animation wrapper |
| `web/components/agent/tooling/ToolPanels.tsx` | SimplePanel: remove card styling; PanelHeader: sentence case; height limits |
| `web/app/globals.css` | Add `.tool-expand-container` grid transition utility |

**No changes needed:**
- `toolRenderers.tsx` — renderer structure stays the same
- `toolDataAdapters.ts` — data layer unchanged
- `toolPresentation.ts` — already good

---

## Execution Order
1. Phase 1 (header polish) — most visible improvement
2. Phase 2 (animation) — feels polished
3. Phase 3 (expanded content) — cleaner detail view
4. Phase 4 (status states) — final touches

---

## Verification
- Visual: check collapsed + expanded states for running/done/error
- Dark mode: verify all color changes work in dark theme
- Responsive: check single vs multi-panel expansion at different widths
- Tests: run `pnpm test` to verify no regressions
- Existing tests in `ToolPanels.test.tsx` should still pass
</content>
</invoke>