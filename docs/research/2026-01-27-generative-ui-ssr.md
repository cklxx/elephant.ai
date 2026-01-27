# Generative UI SSR Research Report
> Date: 2026-01-27

## 1) Intent & Scope
- Goal: Extract generative UI patterns (UI resource delivery, sandboxing, interaction loops) relevant to SSR in our UI stack.
- Scope: MCP Apps announcement post (2026-01-26) and its implications for server-rendered previews.
- Constraints: Focus on generative UI interfaces; avoid MCP protocol specifics unless they inform UI rendering or safety.

## 2) Plan (Multi-round Retrieval)
- Round 1: Read the MCP Apps announcement for UI resource and rendering details.
- Round 2: Identify SSR-relevant elements (resource delivery format, iframe sandboxing, client-host bridge).
- Round 3: Map to our A2UI + Next SSR rendering approach.

## 3) Evidence Table
| Source | Key Data | Date | Relevance |
| --- | --- | --- | --- |
| MCP Apps announcement | Tools can return UI resources, rendered in sandboxed iframes with a UI resource URI; UI-to-host communication via postMessage JSON-RPC; security model emphasizes sandboxing, pre-declared templates, auditable messages, and user consent | 2026-01-26 | Establishes UI resource delivery + sandboxing patterns aligned with SSR HTML previews |

## 4) Analysis & Synthesis
- Generative UI delivery is framed as a UI resource that the host renders directly in the conversation, typically in a sandboxed iframe. This matches our need for a secure rendering surface for model-produced UI, even if we bypass MCP-specific wiring.
- The post emphasizes pre-declared templates, auditable messages, and explicit user consent for UI-initiated actions. For our stack, this suggests SSR previews should remain read-only and sandboxed by default, while interactive flows are handled by the client renderer with explicit event handling.
- Because UI resources are treated as hosted HTML/JS bundles, an SSR HTML preview can serve as a safe, immediate rendering path without trusting dynamic code execution, while still allowing richer interactivity in the client renderer.

## 5) Recommendation / Next Steps
- Use Next.js SSR to render A2UI payloads into a static HTML preview document.
- Serve the SSR HTML through a server route and embed it in a sandboxed iframe.
- Keep the existing client A2UI renderer for interactive use; expose SSR vs interactive tabs in the attachment preview UI.
- Maintain strict SSR input validation and origin allowlists when loading remote payloads.

## 6) Appendix
- MCP Apps announcement: https://blog.modelcontextprotocol.io/posts/2026-01-26-mcp-apps/
