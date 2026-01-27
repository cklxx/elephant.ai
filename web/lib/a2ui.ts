import { AttachmentPayload } from "@/lib/types";
import { loadAttachmentText } from "@/lib/attachment-text";

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
  const text = await loadAttachmentText(attachment, signal);
  if (!text) {
    return [];
  }
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

export { loadAttachmentText };
