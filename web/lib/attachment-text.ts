import { AttachmentPayload } from "@/lib/types";
import { buildAttachmentUri } from "@/lib/attachments";

export async function loadAttachmentText(
  attachment: AttachmentPayload,
  signal?: AbortSignal,
): Promise<string> {
  const rawData = attachment.data?.trim();
  if (rawData) {
    if (rawData.startsWith("{") || rawData.startsWith("[") || rawData.startsWith("\"")) {
      return rawData;
    }
    if (rawData.startsWith("data:")) {
      return decodeDataUri(rawData) ?? "";
    }
    return decodeBase64Text(rawData);
  }

  const uri = buildAttachmentUri(attachment);
  if (!uri) {
    return "";
  }

  if (uri.startsWith("data:")) {
    return decodeDataUri(uri) ?? "";
  }

  const response = await fetch(uri, { signal });
  return await response.text();
}

export function decodeBase64Text(encoded: string): string {
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

export function decodeDataUri(uri: string): string | null {
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
