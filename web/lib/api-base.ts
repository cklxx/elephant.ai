const RAW_API_BASE_URL =
  (typeof import.meta !== "undefined" && import.meta.env?.VITE_API_URL?.trim?.()) ||
  (typeof process !== "undefined" ? process.env.VITE_API_URL?.trim() : undefined);
const DEFAULT_INTERNAL_PRODUCTION_API_BASE = "http://alex-server:8080";
const DEFAULT_DEVELOPMENT_API_BASE = "http://localhost:8080";
const LOCAL_HOSTNAMES = new Set(["localhost", "127.0.0.1", "[::1]"]);

function normalizeBaseUrl(url: string): string {
  return url.replace(/\/$/, "");
}

function isLocalHostname(hostname: string | undefined | null): boolean {
  if (!hostname) {
    return false;
  }

  return LOCAL_HOSTNAMES.has(hostname.trim().toLowerCase());
}

function rewriteLocalhostBaseUrl(url: string): string | null {
  if (typeof window === "undefined" || !window.location) {
    return null;
  }

  const clientHostname = window.location.hostname;
  if (!clientHostname || isLocalHostname(clientHostname)) {
    return null;
  }

  try {
    const parsed = new URL(url);
    if (!isLocalHostname(parsed.hostname)) {
      return null;
    }

    const portSuffix = parsed.port ? `:${parsed.port}` : "";
    const rewritten = `${parsed.protocol}//${clientHostname}${portSuffix}`;

    return normalizeBaseUrl(rewritten);
  } catch {
    return null;
  }
}

export function resolveApiBaseUrl(): string {
  const value = RAW_API_BASE_URL;

  if (!value || value.toLowerCase() === "auto") {
    if (typeof window !== "undefined" && window.location?.origin) {
      return normalizeBaseUrl(window.location.origin);
    }

    const nodeEnv = typeof process !== "undefined" ? process.env.NODE_ENV : undefined;
    return nodeEnv === "production"
      ? normalizeBaseUrl(DEFAULT_INTERNAL_PRODUCTION_API_BASE)
      : normalizeBaseUrl(DEFAULT_DEVELOPMENT_API_BASE);
  }

  const normalized = normalizeBaseUrl(value);
  const rewritten = rewriteLocalhostBaseUrl(normalized);

  return rewritten ?? normalized;
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
