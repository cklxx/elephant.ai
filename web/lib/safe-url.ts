const URL_SCHEME_PATTERN = /^([a-z][a-z0-9+.-]*):/i;
const SAFE_LINK_SCHEMES = new Set(["http", "https", "mailto", "tel"]);
const SAFE_MEDIA_SCHEMES = new Set(["http", "https", "data", "blob"]);

function containsUnsafeControlChars(value: string): boolean {
  return /[\u0000-\u001F\u007F]/.test(value);
}

function hasRelativeUrlPrefix(value: string): boolean {
  return (
    value.startsWith("/") ||
    value.startsWith("./") ||
    value.startsWith("../") ||
    value.startsWith("?") ||
    value.startsWith("#")
  );
}

function extractScheme(value: string): string | null {
  const normalized = value.replace(/[\t\n\r\f ]+/g, "");
  const match = URL_SCHEME_PATTERN.exec(normalized);
  return match?.[1]?.toLowerCase() ?? null;
}

function sanitizeUrlWithAllowedSchemes(
  value: string,
  allowedSchemes: Set<string>,
): string | null {
  const trimmed = value.trim();
  if (!trimmed || containsUnsafeControlChars(trimmed)) {
    return null;
  }
  if (trimmed.startsWith("//") || trimmed.startsWith("\\\\")) {
    return null;
  }
  if (hasRelativeUrlPrefix(trimmed)) {
    return trimmed;
  }

  const scheme = extractScheme(trimmed);
  if (!scheme) {
    return trimmed;
  }

  return allowedSchemes.has(scheme) ? trimmed : null;
}

export function sanitizeLinkHref(value: string): string | null {
  return sanitizeUrlWithAllowedSchemes(value, SAFE_LINK_SCHEMES);
}

export function sanitizeMediaUrl(value: string): string | null {
  return sanitizeUrlWithAllowedSchemes(value, SAFE_MEDIA_SCHEMES);
}
