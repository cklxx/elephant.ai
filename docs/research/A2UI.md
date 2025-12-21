# A2UI Research Notes

Last updated: 2025-12-21

## Summary
A2UI (Agent-to-User Interface) is an open protocol and set of libraries that let
agents send declarative UI descriptions instead of executable code. The UI is
expressed as a JSONL stream of messages (typically over SSE) that build a
platform-agnostic component tree and data model, which the client renders with a
local catalog of trusted widgets.

Core idea: "safe like data, expressive like code" by restricting agents to a
catalog of pre-approved components and separating UI structure from data.

## Problems A2UI Solves
- Security across trust boundaries: avoid executing model-generated code.
- Cross-platform rendering: the same UI description renders on web, Flutter,
  native, etc., via the client-side catalog.
- LLM-friendly generation: flat component list + references are easier to
  generate than nested trees.
- Progressive UX: JSONL streaming + beginRendering allow immediate partial UI.
- Incremental updates: small dataModelUpdate patches avoid full re-render.

## Protocol Overview (v0.8)
Transport: JSON Lines (JSONL), typically via SSE.
Each message MUST contain exactly one of:
- beginRendering
- surfaceUpdate
- dataModelUpdate
- deleteSurface

Key concepts:
- Surface: a named render target (surfaceId) with its own component tree and
  data model.
- Component list: flat adjacency list, components reference child IDs.
- Data model: structured values, updated via dataModelUpdate.
- Catalog: client-side registry of available components and their schemas.

### Message Types
- surfaceUpdate: sends component definitions to store in the surface map.
- dataModelUpdate: updates data for a surface (root or path).
- beginRendering: sets the root component ID and optional styles/catalogId.
- deleteSurface: removes a surface and its UI.

### Data Binding
Components bind to data via BoundValue objects:
- literal* (static value)
- path (data model path)
- optional literal + path to initialize + bind in one step

### Client Events (A2A)
User interactions are sent back via a separate client-to-server event message:
- userAction with name, surfaceId, sourceComponentId, and resolved context
- error for client rendering failures

## Catalog Negotiation
- Server advertises supportedCatalogIds and acceptsInlineCatalogs.
- Client includes supportedCatalogIds (and optional inline catalogs) in each
  request.
- Server chooses a catalog and references catalogId in beginRendering.

## Security Model
- Only known component types from a local catalog are renderable.
- No remote code execution; rendering is controlled by client.
- Custom components can enforce additional sandboxing and policies.

## Rendering Flow (Client)
1. Receive JSONL messages and buffer components + data model.
2. Wait for beginRendering to render root.
3. Resolve component tree by ID references.
4. Resolve data bindings.
5. Render with local widget registry.
6. Emit userAction events for interactions.

## Integration Considerations for This Repo
- Existing SSE event pipeline can carry A2UI payloads as attachments/artifacts.
- Frontend already renders attachments via ArtifactPreviewCard; A2UI fits as a
  new attachment preview profile (e.g., document.a2ui).
- Tooling can emit A2UI payloads as attachments and let UI render them without
  altering the primary event stream.

## Risks / Limitations
- Catalog mismatch: server must align with client catalog.
- Large component streams can be heavy; JSONL chunking is needed.
- Some components require custom UX handling (e.g., complex inputs, validation).
- Spec is still in public preview (v0.8), may evolve.

## References
- Google Developers Blog: Introducing A2UI
  https://developers.googleblog.com/en/introducing-a2ui-an-open-project-for-agent-driven-interfaces/
- A2UI README
  https://github.com/google/A2UI
- A2UI Protocol (v0.8)
  https://github.com/google/A2UI/blob/main/specification/0.8/docs/a2ui_protocol.md
- Standard Catalog Schema (v0.8)
  https://github.com/google/A2UI/blob/main/specification/0.8/json/standard_catalog_definition.json
