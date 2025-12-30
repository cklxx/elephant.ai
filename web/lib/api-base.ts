const RAW_API_BASE_URL = process.env.NEXT_PUBLIC_API_URL?.trim();
const DEFAULT_INTERNAL_PRODUCTION_API_BASE = "http://alex-server:8080";
const DEFAULT_DEVELOPMENT_API_BASE = "http://localhost:8080";
const LOCAL_HOSTNAMES = new Set(["localhost", "127.0.0.1", "[::1]"]);

const INTERNAL_API_HOSTS = new Set(["alex-server"]);

function normalizeBaseUrl(url: string): string {
  return url.replace(/\/$/, "");
}

function isLocalHostname(hostname: string | undefined | null): boolean {
  if (!hostname) {
    return false;
  }

  return LOCAL_HOSTNAMES.has(hostname.trim().toLowerCase());
}

function isInternalApiHostname(hostname: string | undefined | null): boolean {
  if (!hostname) {
    return false;
  }

  const normalized = hostname.trim().toLowerCase();
  return LOCAL_HOSTNAMES.has(normalized) || INTERNAL_API_HOSTS.has(normalized);
}

function rewriteInternalBaseUrl(url: string): string | null {
  if (typeof window === "undefined" || !window.location) {
    return null;
  }

  const clientHostname = window.location.hostname;
  const clientProtocol = window.location.protocol;
  if (!clientHostname || isInternalApiHostname(clientHostname)) {
    return null;
  }

  try {
    const parsed = new URL(url);
    if (!isInternalApiHostname(parsed.hostname)) {
      return null;
    }

    const portSuffix = parsed.port ? `:${parsed.port}` : "";
    const protocol = clientProtocol || parsed.protocol;
    const rewritten = `${protocol}//${clientHostname}${portSuffix}`;

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

    return process.env.NODE_ENV === "production"
      ? normalizeBaseUrl(DEFAULT_INTERNAL_PRODUCTION_API_BASE)
      : normalizeBaseUrl(DEFAULT_DEVELOPMENT_API_BASE);
  }

  const normalized = normalizeBaseUrl(value);
  const rewritten = rewriteInternalBaseUrl(normalized);

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
