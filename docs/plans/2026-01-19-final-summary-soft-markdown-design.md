# Final Summary Soft-Structured Rendering Design

## Goal
Make the final summary feel like concise prose instead of strongly structured Markdown, while preserving emphasis and attachment rendering. The UI should soften headings and lists into paragraph-like flow, and the prompt should guide the model to produce shorter, less structured summaries when documents are provided.

## Scope
Applies only to the final answer card. Intermediate events and tool outputs keep current Markdown rendering. The data format of events and attachments stays unchanged.

## Architecture
The final answer is rendered by `TaskCompleteCard` using `AgentMarkdown`. We will inject a dedicated component override set for this card only. These overrides will remap headings and lists into paragraph-like containers and keep `strong` emphasis. Inline media substitution and attachment previews will continue to use the current logic, so existing image/video/document rendering remains intact.

## Rendering Behavior
Headings (`h1`-`h6`) render as normal blocks with modest emphasis but no large typography. Ordered and unordered lists render as stacked blocks without bullets or numbering; list items render as paragraph blocks. Horizontal rules and blockquotes are softened into lightly separated paragraphs. Strong emphasis is retained and slightly strengthened to keep key phrases visually prominent. Streaming behavior and placeholder handling remain the same.

## Prompt Adjustment
Update the default persona voice to explicitly request short, paragraph-based final summaries with minimal headings or lists. When document attachments are present, the summary should stay concise and point to the document rather than repeating details.

## Error Handling
No new error paths are introduced. If the renderer encounters malformed Markdown, the existing renderer behavior remains; only the component mapping changes.

## Testing
Add a UI test for `TaskCompleteCard` that supplies a final answer with headings and lists and verifies that the rendered output contains the text but not `h1`/`ul`/`ol` elements, while `strong` emphasis remains. Run targeted frontend tests and the full lint/test suite as required.

## Risks and Mitigations
Soft rendering reduces Markdown semantics in the final card, which may remove explicit list markers. This is intentional for a more narrative summary. We keep attachment previews and inline media replacements untouched to avoid regression in media rendering.
