import { A2UIMessage, parseA2UIMessagePayload } from "@/lib/a2ui";
import { JsonRenderTree, parseJsonRenderPayload } from "@/lib/json-render-model";

export type UIPayload =
  | { kind: "a2ui"; messages: A2UIMessage[] }
  | { kind: "json-render"; tree: JsonRenderTree }
  | { kind: "unknown"; error: string };

export function parseUIPayload(payload: string): UIPayload {
  const trimmed = typeof payload === "string" ? payload.trim() : "";
  if (!trimmed) {
    return { kind: "unknown", error: "Empty UI payload." };
  }

  try {
    const messages = parseA2UIMessagePayload(trimmed);
    if (hasA2UIFields(messages)) {
      return { kind: "a2ui", messages };
    }
  } catch {
    // Ignore and fall through to json-render parsing.
  }

  try {
    const tree = parseJsonRenderPayload(trimmed);
    if (tree.root) {
      return { kind: "json-render", tree };
    }
    return { kind: "unknown", error: "No renderable UI content." };
  } catch (err) {
    return {
      kind: "unknown",
      error: err instanceof Error ? err.message : String(err),
    };
  }
}

function hasA2UIFields(messages: A2UIMessage[]): boolean {
  return messages.some(
    (message) =>
      Boolean(
        message.surfaceUpdate ||
          message.dataModelUpdate ||
          message.beginRendering ||
          message.deleteSurface,
      ),
  );
}
