import { AttachmentPayload } from "@/lib/types";
import { buildAttachmentUri } from "@/lib/attachments";

export type A2UIMessage = {
  surfaceUpdate?: {
    surfaceId?: string;
    components?: Array<{
      id?: string;
      weight?: number;
      component?: Record<string, any>;
    }>;
  };
  dataModelUpdate?: {
    surfaceId?: string;
    path?: string;
    contents?: any;
  };
  beginRendering?: {
    surfaceId?: string;
    catalogId?: string;
    root?: string;
    styles?: Record<string, any>;
  };
  deleteSurface?: {
    surfaceId?: string;
  };
};

export function parseA2UIMessagePayload(payload: string): A2UIMessage[] {
  const trimmed = typeof payload === "string" ? payload.trim() : "";
  if (!trimmed) {
    return [];
  }

  try {
    const parsed = JSON.parse(trimmed);
    return normalizeA2UIMessages(parsed);
  } catch {
    // Fall through to JSONL parsing.
  }

  const messages: A2UIMessage[] = [];
  const lines = trimmed.split(/\r?\n/);
  const errors: string[] = [];
  for (const line of lines) {
    const candidate = line.trim();
    if (!candidate) {
      continue;
    }
    try {
      const parsed = JSON.parse(candidate);
      messages.push(...normalizeA2UIMessages(parsed));
    } catch (err) {
      errors.push(String(err));
    }
  }

  if (messages.length === 0 && errors.length > 0) {
    throw new Error("Unable to parse A2UI payload.");
  }
  return messages;
}

export async function loadA2UIAttachmentMessages(
  attachment: AttachmentPayload,
  signal?: AbortSignal,
): Promise<A2UIMessage[]> {
  const rawData = attachment.data?.trim();
  if (rawData) {
    if (rawData.startsWith("{") || rawData.startsWith("[")) {
      return parseA2UIMessagePayload(rawData);
    }
    if (rawData.startsWith("data:")) {
      const decoded = decodeDataUri(rawData);
      return parseA2UIMessagePayload(decoded ?? "");
    }
    return parseA2UIMessagePayload(decodeBase64Text(rawData));
  }

  const uri = buildAttachmentUri(attachment);
  if (!uri) {
    return [];
  }

  if (uri.startsWith("data:")) {
    const decoded = decodeDataUri(uri);
    return parseA2UIMessagePayload(decoded ?? "");
  }

  const response = await fetch(uri, { signal });
  const text = await response.text();
  return parseA2UIMessagePayload(text);
}

function normalizeA2UIMessages(value: any): A2UIMessage[] {
  if (Array.isArray(value)) {
    return value.filter(isPlainObject) as A2UIMessage[];
  }
  if (isPlainObject(value)) {
    return [value as A2UIMessage];
  }
  return [];
}

function isPlainObject(value: unknown): value is Record<string, any> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function decodeBase64Text(encoded: string): string {
  if (typeof atob === "function") {
    const binary = atob(encoded);
    const bytes = Uint8Array.from(binary, (char) => char.charCodeAt(0));
    return new TextDecoder("utf-8").decode(bytes);
  }
  const buffer = (globalThis as any).Buffer;
  if (buffer) {
    return buffer.from(encoded, "base64").toString("utf-8");
  }
  return encoded;
}

function decodeDataUri(uri: string): string | null {
  const match = uri.match(/^data:([^,]*?),(.*)$/);
  if (!match) {
    return null;
  }
  const meta = match[1] || "";
  const data = match[2] || "";
  if (meta.includes(";base64")) {
    return decodeBase64Text(data);
  }
  try {
    return decodeURIComponent(data);
  } catch {
    return data;
  }
}
