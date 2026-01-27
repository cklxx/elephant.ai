import { JsonRenderTree, parseJsonRenderPayload } from "@/lib/json-render-model";

export type UIPayload =
  | { kind: "json-render"; tree: JsonRenderTree }
  | { kind: "unknown"; error: string };

export function parseUIPayload(payload: string): UIPayload {
  const trimmed = typeof payload === "string" ? payload.trim() : "";
  if (!trimmed) {
    return { kind: "unknown", error: "Empty UI payload." };
  }

  if (looksLikeA2UI(trimmed)) {
    return {
      kind: "unknown",
      error: "A2UI payloads are not supported. Use json-render payloads instead.",
    };
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

function looksLikeA2UI(payload: string): boolean {
  const trimmed = payload.trim();
  if (!trimmed) {
    return false;
  }

  const parsed = parseJSONValue(trimmed);
  if (parsed !== undefined) {
    return isA2UIValue(parsed);
  }

  const lines = trimmed.split(/\r?\n/);
  for (const line of lines) {
    const candidate = line.trim();
    if (!candidate) {
      continue;
    }
    const parsedLine = parseJSONValue(candidate);
    if (parsedLine !== undefined && isA2UIValue(parsedLine)) {
      return true;
    }
  }
  return false;
}

function parseJSONValue(value: string): any | undefined {
  try {
    return JSON.parse(value);
  } catch {
    return undefined;
  }
}

function isA2UIValue(value: any): boolean {
  if (Array.isArray(value)) {
    return value.some(isA2UIValue);
  }
  if (!isPlainObject(value)) {
    return false;
  }
  return Boolean(
    value.surfaceUpdate ||
      value.dataModelUpdate ||
      value.beginRendering ||
      value.deleteSurface,
  );
}

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}
