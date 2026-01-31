import { buildApiUrl } from "@/lib/api-base";
import type { AttachmentPayload } from "@/lib/types";

type PreviewAsset = NonNullable<AttachmentPayload["preview_assets"]>[number];

const DEFAULT_BLOB_URL_CACHE_LIMIT = 200;
const BLOB_URL_CACHE_LIMIT = (() => {
  const raw =
    typeof process !== "undefined"
      ? Number(process.env.NEXT_PUBLIC_BLOB_URL_CACHE_LIMIT)
      : Number.NaN;
  return Number.isFinite(raw) && raw > 0 ? raw : DEFAULT_BLOB_URL_CACHE_LIMIT;
})();
const BLOB_URL_CACHE = new Map<string, string>();
const PRESIGNED_REFRESH_WINDOW_MS = 5 * 60 * 1000;
const PRESIGNED_FILENAME_PATTERN = /^[a-f0-9]{64}(\.[a-z0-9]{1,10})?$/i;

function hashValue(value: string): string {
  let hash = 2166136261;
  for (let i = 0; i < value.length; i += 1) {
    hash ^= value.charCodeAt(i);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(16).padStart(8, "0");
}

function rememberBlobUrl(cacheKey: string, url: string): void {
  if (BLOB_URL_CACHE.has(cacheKey)) {
    BLOB_URL_CACHE.delete(cacheKey);
  }
  BLOB_URL_CACHE.set(cacheKey, url);

  if (BLOB_URL_CACHE.size <= BLOB_URL_CACHE_LIMIT) {
    return;
  }

  const revoke =
    typeof URL !== "undefined" && typeof URL.revokeObjectURL === "function"
      ? URL.revokeObjectURL.bind(URL)
      : null;
  while (BLOB_URL_CACHE.size > BLOB_URL_CACHE_LIMIT) {
    const oldest = BLOB_URL_CACHE.entries().next().value as
      | [string, string]
      | undefined;
    if (!oldest) {
      return;
    }
    const [oldKey, oldUrl] = oldest;
    BLOB_URL_CACHE.delete(oldKey);
    if (revoke) {
      try {
        revoke(oldUrl);
      } catch {
        // Ignore revoke errors to avoid breaking rendering paths.
      }
    }
  }
}

function toTrimmedString(value: unknown): string | undefined {
  return typeof value === "string" ? value.trim() : undefined;
}

function canCreateBlobUrl(): boolean {
  return (
    typeof URL !== "undefined" &&
    typeof URL.createObjectURL === "function" &&
    typeof Blob !== "undefined"
  );
}

function decodeBase64ToBytes(value: string): Uint8Array | null {
  try {
    const binary =
      typeof atob === "function"
        ? atob(value)
        : typeof Buffer !== "undefined"
          ? Buffer.from(value, "base64").toString("binary")
          : null;
    if (!binary) {
      return null;
    }
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  } catch {
    return null;
  }
}

function buildBlobUrlFromBytes(
  cacheKey: string,
  bytes: Uint8Array,
  mediaType: string,
): string | null {
  if (!canCreateBlobUrl()) {
    return null;
  }
  const existing = BLOB_URL_CACHE.get(cacheKey);
  if (existing) {
    rememberBlobUrl(cacheKey, existing);
    return existing;
  }
  try {
    const payload = new Uint8Array(bytes).buffer;
    const blob = new Blob([payload], { type: mediaType });
    const url = URL.createObjectURL(blob);
    rememberBlobUrl(cacheKey, url);
    return url;
  } catch {
    return null;
  }
}

function buildBlobUrlFromBase64(
  base64: string,
  mediaType: string,
): string | null {
  if (!canCreateBlobUrl()) {
    return null;
  }
  const cacheKey = `base64:${mediaType}:${base64.length}:${hashValue(base64)}`;
  const bytes = decodeBase64ToBytes(base64);
  if (!bytes) {
    return null;
  }
  return buildBlobUrlFromBytes(cacheKey, bytes, mediaType);
}

function buildBlobUrlFromDataUri(dataUri: string): string | null {
  if (!canCreateBlobUrl()) {
    return null;
  }
  if (!dataUri.startsWith("data:")) {
    return null;
  }
  const cacheKey = `data:${dataUri.length}:${hashValue(dataUri)}`;
  const cached = BLOB_URL_CACHE.get(cacheKey);
  if (cached) {
    rememberBlobUrl(cacheKey, cached);
    return cached;
  }
  const match = /^data:([^,]*),(.*)$/.exec(dataUri);
  if (!match) {
    return null;
  }
  const meta = match[1] ?? "";
  const payload = match[2] ?? "";
  const isBase64 = /;base64/i.test(meta);
  const mediaType = meta.split(";")[0] || "application/octet-stream";
  if (isBase64) {
    const bytes = decodeBase64ToBytes(payload);
    if (!bytes) {
      return null;
    }
    return buildBlobUrlFromBytes(cacheKey, bytes, mediaType);
  }

  try {
    const decoded = decodeURIComponent(payload);
    const encoder = typeof TextEncoder !== "undefined" ? new TextEncoder() : null;
    const bytes = encoder ? encoder.encode(decoded) : new Uint8Array([]);
    if (bytes.length === 0) {
      return null;
    }
    return buildBlobUrlFromBytes(cacheKey, bytes, mediaType);
  } catch {
    return null;
  }
}

function normalizeAttachmentUri(uri: string): string {
  const trimmed = uri.trim();
  if (trimmed.startsWith("data:")) {
    return trimmed;
  }
  if (trimmed.startsWith("http://") || trimmed.startsWith("https://")) {
    const refreshed = refreshPresignedAttachmentUrl(trimmed);
    return refreshed ?? trimmed;
  }
  if (trimmed.startsWith("/")) {
    return buildApiUrl(trimmed);
  }
  return trimmed;
}

function refreshPresignedAttachmentUrl(uri: string): string | null {
  let parsed: URL;
  try {
    parsed = new URL(uri);
  } catch {
    return null;
  }

  const amzDate = getQueryParam(parsed, "x-amz-date");
  const amzExpires = getQueryParam(parsed, "x-amz-expires");
  if (!amzDate || !amzExpires) {
    return null;
  }
  const expirySeconds = Number(amzExpires);
  if (!Number.isFinite(expirySeconds) || expirySeconds <= 0) {
    return null;
  }
  const issuedAt = parseAmzDate(amzDate);
  if (issuedAt === null) {
    return null;
  }

  const expiresAt = issuedAt + expirySeconds * 1000;
  if (expiresAt - Date.now() > PRESIGNED_REFRESH_WINDOW_MS) {
    return null;
  }

  const filename = parsed.pathname.split("/").filter(Boolean).pop();
  if (!filename || !PRESIGNED_FILENAME_PATTERN.test(filename)) {
    return null;
  }
  return buildApiUrl(`/api/attachments/${filename}`);
}

function parseAmzDate(value: string): number | null {
  const trimmed = value.trim();
  const match = /^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})Z$/.exec(
    trimmed,
  );
  if (!match) {
    return null;
  }
  const [, year, month, day, hour, minute, second] = match;
  const timestamp = Date.UTC(
    Number(year),
    Number(month) - 1,
    Number(day),
    Number(hour),
    Number(minute),
    Number(second),
  );
  return Number.isNaN(timestamp) ? null : timestamp;
}

function getQueryParam(url: URL, key: string): string | null {
  const normalized = key.toLowerCase();
  for (const [paramKey, value] of url.searchParams.entries()) {
    if (paramKey.toLowerCase() === normalized) {
      return value;
    }
  }
  return null;
}

function isPresentationAttachment(attachment: AttachmentPayload): boolean {
  const format = attachment.format?.toLowerCase() ?? "";
  const mediaType = attachment.media_type?.toLowerCase() ?? "";
  return (
    format === "ppt" ||
    format === "pptx" ||
    mediaType.includes("officedocument.presentation") ||
    mediaType === "application/ppt" ||
    mediaType === "application/powerpoint"
  );
}

function findPreferredDownloadAsset(
  attachment: AttachmentPayload,
): PreviewAsset | undefined {
  if (!isPresentationAttachment(attachment)) {
    return undefined;
  }

  const previewAssets = attachment.preview_assets ?? [];
  return previewAssets.find((asset) => {
    const mime = toTrimmedString(asset.mime_type)?.toLowerCase() ?? "";
    const previewType = toTrimmedString(asset.preview_type)?.toLowerCase() ?? "";
    const cdnUrl = toTrimmedString(asset.cdn_url);
    if (!cdnUrl) {
      return false;
    }
    return mime === "application/pdf" || previewType.includes("pdf");
  });
}

function buildAttachmentUriInternal(
  attachment: AttachmentPayload,
  options?: { preferDownloadAsset?: boolean },
): string | null {
  const preferredAsset =
    options?.preferDownloadAsset === false
      ? undefined
      : findPreferredDownloadAsset(attachment);
  if (preferredAsset?.cdn_url) {
    return normalizeAttachmentUri(preferredAsset.cdn_url);
  }

  const direct = toTrimmedString(attachment.uri);
  if (direct) {
    if (direct.startsWith("data:")) {
      const blobUrl = buildBlobUrlFromDataUri(direct);
      if (blobUrl) {
        return blobUrl;
      }
    }
    return normalizeAttachmentUri(direct);
  }
  const data = toTrimmedString(attachment.data);
  if (!data) {
    const previewAssets = attachment.preview_assets ?? [];
    const preferredAsset = previewAssets.find((asset) => {
      const mime = toTrimmedString(asset.mime_type)?.toLowerCase() ?? "";
      const previewType = toTrimmedString(asset.preview_type)?.toLowerCase() ?? "";
      return (
        Boolean(toTrimmedString(asset.cdn_url)) &&
        (mime.startsWith("video/") || previewType.includes("video"))
      );
    });
    const fallbackAsset = previewAssets.find((asset) =>
      toTrimmedString(asset.cdn_url),
    );
    const cdnUrl =
      toTrimmedString(preferredAsset?.cdn_url) ||
      toTrimmedString(fallbackAsset?.cdn_url);
    return cdnUrl ? normalizeAttachmentUri(cdnUrl) : null;
  }
  if (
    data.startsWith("data:") ||
    data.startsWith("http://") ||
    data.startsWith("https://") ||
    data.startsWith("/")
  ) {
    if (data.startsWith("data:")) {
      const blobUrl = buildBlobUrlFromDataUri(data);
      if (blobUrl) {
        return blobUrl;
      }
    }
    return normalizeAttachmentUri(data);
  }
  const mediaType = attachment.media_type?.trim() || "application/octet-stream";
  const blobUrl = buildBlobUrlFromBase64(data, mediaType);
  if (blobUrl) {
    return blobUrl;
  }
  return `data:${mediaType};base64,${data}`;
}

export function buildAttachmentUri(attachment: AttachmentPayload): string | null {
  return buildAttachmentUriInternal(attachment, { preferDownloadAsset: true });
}

export function resolveAttachmentDownloadUris(
  attachment: AttachmentPayload,
): { preferredUri: string | null; fallbackUri: string | null; preferredKind: "pdf" | null } {
  const preferredAsset = findPreferredDownloadAsset(attachment);
  const fallbackUri = buildAttachmentUriInternal(attachment, {
    preferDownloadAsset: false,
  });

  if (preferredAsset?.cdn_url) {
    const preferredUri = normalizeAttachmentUri(preferredAsset.cdn_url);
    const fallbackDistinct =
      fallbackUri && fallbackUri !== preferredUri ? fallbackUri : null;
    return { preferredUri, fallbackUri: fallbackDistinct, preferredKind: "pdf" };
  }

  return { preferredUri: fallbackUri, fallbackUri: null, preferredKind: null };
}
