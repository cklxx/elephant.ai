const RAW_API_BASE_URL = process.env.NEXT_PUBLIC_API_URL?.trim();
const DEFAULT_INTERNAL_PRODUCTION_API_BASE = "http://alex-server:8080";
const DEFAULT_DEVELOPMENT_API_BASE = "http://localhost:8080";

function normalizeBaseUrl(url: string): string {
  return url.replace(/\/$/, "");
}

export function resolveApiBaseUrl(): string {
  const value = RAW_API_BASE_URL;

  if (!value || value.toLowerCase() === "auto") {
    if (typeof window !== "undefined" && window.location?.origin) {
      return normalizeBaseUrl(window.location.origin);
    }

    return process.env.NODE_ENV === "production"
      ? normalizeBaseUrl(DEFAULT_INTERNAL_PRODUCTION_API_BASE)
      : normalizeBaseUrl(DEFAULT_DEVELOPMENT_API_BASE);
  }

  return normalizeBaseUrl(value);
}

export function buildApiUrl(endpoint: string): string {
  const baseUrl = resolveApiBaseUrl();

  if (!baseUrl) {
    return endpoint;
  }

  if (endpoint.startsWith("/")) {
    return `${baseUrl}${endpoint}`;
  }

  return `${baseUrl}/${endpoint}`;
}
