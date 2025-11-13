import { buildApiUrl } from "../api-base";

type RawSubscription = {
  tier: string;
  monthly_price_cents: number;
  expires_at?: string | null;
};

type RawSubscriptionPlan = {
  tier: string;
  monthly_price_cents: number;
};

type RawAuthUser = {
  id: string;
  email: string;
  display_name: string;
  points_balance?: number;
  subscription?: RawSubscription;
};

type TokenResponse = {
  access_token: string;
  expires_at: string;
  refresh_expires_at: string;
  user: RawAuthUser;
};

type OAuthStartResponse = {
  url?: string;
  state?: string;
};

export type OAuthProvider = "google" | "wechat";

export interface OAuthSessionOptions {
  popup?: Window | null;
  timeoutMs?: number;
  pollIntervalMs?: number;
  signal?: AbortSignal;
}

export interface SubscriptionInfo {
  tier: string;
  monthlyPriceCents: number;
  expiresAt: string | null;
  isPaid: boolean;
}

export interface SubscriptionPlan {
  tier: string;
  monthlyPriceCents: number;
  isPaid: boolean;
}

export interface AuthUser {
  id: string;
  email: string;
  displayName: string;
  pointsBalance: number;
  subscription: SubscriptionInfo;
}

export interface AuthSession {
  accessToken: string;
  accessExpiry: string;
  refreshExpiry: string;
  user: AuthUser;
}

type SessionListener = (session: AuthSession | null) => void;

const STORAGE_KEY = "alex.console.auth";
const EXPIRY_BUFFER_MS = 30 * 1000; // 30 seconds safety buffer

function sessionsEqual(a: AuthSession | null, b: AuthSession | null): boolean {
  if (a === b) {
    return true;
  }

  if (!a || !b) {
    return false;
  }

  if (
    a.accessToken !== b.accessToken ||
    a.accessExpiry !== b.accessExpiry ||
    a.refreshExpiry !== b.refreshExpiry
  ) {
    return false;
  }

  const userA = a.user;
  const userB = b.user;

  if (
    userA.id !== userB.id ||
    userA.email !== userB.email ||
    userA.displayName !== userB.displayName ||
    userA.pointsBalance !== userB.pointsBalance
  ) {
    return false;
  }

  const subA = userA.subscription;
  const subB = userB.subscription;

  return (
    subA.tier === subB.tier &&
    subA.monthlyPriceCents === subB.monthlyPriceCents &&
    subA.expiresAt === subB.expiresAt &&
    subA.isPaid === subB.isPaid
  );
}

function isBrowser(): boolean {
  return (
    typeof window !== "undefined" && typeof window.localStorage !== "undefined"
  );
}

function parseISODate(value: string): number {
  const time = Date.parse(value);
  return Number.isNaN(time) ? 0 : time;
}

function hasExpired(iso: string, bufferMs = 0): boolean {
  if (!iso) {
    return true;
  }
  const timestamp = parseISODate(iso);
  if (!timestamp) {
    return true;
  }
  return timestamp <= Date.now() + bufferMs;
}

function mapResponse(response: TokenResponse): AuthSession {
  const rawUser = response.user;
  return {
    accessToken: response.access_token,
    accessExpiry: response.expires_at,
    refreshExpiry: response.refresh_expires_at,
    user: mapUser(rawUser),
  };
}

function normalizePoints(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return Math.max(0, Math.trunc(value));
  }
  return 0;
}

function mapUser(rawUser: RawAuthUser): AuthUser {
  return {
    id: rawUser.id,
    email: rawUser.email,
    displayName: rawUser.display_name,
    pointsBalance: normalizePoints(rawUser.points_balance),
    subscription: mapSubscription(rawUser.subscription),
  };
}

function mapSubscription(raw: RawSubscription | undefined): SubscriptionInfo {
  const tier = typeof raw?.tier === "string" && raw.tier.trim() ? raw.tier : "free";
  const monthlyPrice =
    typeof raw?.monthly_price_cents === "number" && Number.isFinite(raw.monthly_price_cents)
      ? Math.max(0, Math.trunc(raw.monthly_price_cents))
      : 0;
  const expiresAt = raw?.expires_at ?? null;
  return {
    tier,
    monthlyPriceCents: monthlyPrice,
    expiresAt,
    isPaid: monthlyPrice > 0,
  };
}

function mapSubscriptionPlan(raw: RawSubscriptionPlan): SubscriptionPlan {
  const tier = typeof raw?.tier === "string" && raw.tier.trim() ? raw.tier : "free";
  const monthlyPrice =
    typeof raw?.monthly_price_cents === "number" && Number.isFinite(raw.monthly_price_cents)
      ? Math.max(0, Math.trunc(raw.monthly_price_cents))
      : 0;
  return {
    tier,
    monthlyPriceCents: monthlyPrice,
    isPaid: monthlyPrice > 0,
  };
}

function isValidSessionShape(session: AuthSession | null): session is AuthSession {
  if (!session || typeof session !== "object") {
    return false;
  }
  const { accessToken, accessExpiry, refreshExpiry, user } = session as AuthSession;
  if (
    typeof accessToken !== "string" ||
    typeof accessExpiry !== "string" ||
    typeof refreshExpiry !== "string" ||
    !user ||
    typeof user !== "object" ||
    typeof user.email !== "string"
  ) {
    return false;
  }
  return true;
}

function normalizeStoredSession(session: AuthSession): AuthSession {
  const existingSubscription = session.user.subscription as Partial<SubscriptionInfo> | undefined;
  const normalizedSubscription = mapSubscription({
    tier: typeof existingSubscription?.tier === "string" ? existingSubscription.tier : "free",
    monthly_price_cents:
      typeof existingSubscription?.monthlyPriceCents === "number"
        ? existingSubscription.monthlyPriceCents
        : existingSubscription?.isPaid
        ? 1
        : 0,
    expires_at: existingSubscription?.expiresAt ?? null,
  });

  return {
    ...session,
    user: {
      ...session.user,
      pointsBalance: normalizePoints((session.user as Partial<AuthUser>).pointsBalance),
      subscription: normalizedSubscription,
    },
  };
}

function readSessionFromStorage(): AuthSession | null {
  if (!isBrowser()) {
    return null;
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) {
      return null;
    }
    const parsed = JSON.parse(raw) as AuthSession;
    if (!isValidSessionShape(parsed)) {
      return null;
    }
    return normalizeStoredSession(parsed);
  } catch (error) {
    console.warn("[authClient] Failed to read session from storage", error);
    return null;
  }
}

function writeSessionToStorage(session: AuthSession | null): void {
  if (!isBrowser()) {
    return;
  }
  try {
    if (!session) {
      window.localStorage.removeItem(STORAGE_KEY);
      return;
    }
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  } catch (error) {
    console.warn("[authClient] Failed to persist session", error);
  }
}

async function postJSON<T>(
  endpoint: string,
  body?: Record<string, unknown>,
  init?: { headers?: Record<string, string> },
): Promise<T> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
    ...(init?.headers ?? {}),
  };
  const response = await fetch(buildApiUrl(endpoint), {
    method: "POST",
    credentials: "include",
    headers,
    body: body ? JSON.stringify(body) : undefined,
  });
  return parseJSONResponse<T>(response);
}

async function getJSON<T>(
  endpoint: string,
  init?: { headers?: Record<string, string> },
): Promise<T> {
  const response = await fetch(buildApiUrl(endpoint), {
    method: "GET",
    credentials: "include",
    headers: init?.headers,
  });
  return parseJSONResponse<T>(response);
}

async function parseJSONResponse<T>(response: Response): Promise<T> {
  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  if (!response.ok) {
    const message = text?.trim() || `HTTP ${response.status}`;
    throw new Error(message);
  }

  if (!text) {
    return undefined as T;
  }

  return JSON.parse(text) as T;
}

class AuthClient {
  private session: AuthSession | null = readSessionFromStorage();
  private listeners = new Set<SessionListener>();
  private refreshPromise: Promise<AuthSession | null> | null = null;
  private storageSyncInProgress = false;

  getSession(): AuthSession | null {
    return this.session;
  }

  subscribe(listener: SessionListener): () => void {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }

  private notify(session: AuthSession | null): void {
    for (const listener of this.listeners) {
      try {
        listener(session);
      } catch (error) {
        console.error("[authClient] Session listener failed", error);
      }
    }
  }

  private setSession(session: AuthSession | null): void {
    this.session = session;
    writeSessionToStorage(session);
    this.notify(session);
  }

  private updateSessionUser(user: AuthUser): void {
    if (!this.session) {
      return;
    }
    const nextSession: AuthSession = {
      ...this.session,
      user,
    };
    this.setSession(nextSession);
  }

  clearSession(): void {
    this.setSession(null);
  }

  handleStorageEvent(event: StorageEvent): void {
    if (typeof window === "undefined") {
      return;
    }

    if (event.key !== STORAGE_KEY) {
      return;
    }

    if (this.storageSyncInProgress) {
      return;
    }

    try {
      this.storageSyncInProgress = true;
      const stored = readSessionFromStorage();
      if (sessionsEqual(stored, this.session)) {
        return;
      }
      this.session = stored;
      this.notify(this.session);
    } catch (error) {
      console.warn("[authClient] Failed to synchronize session from storage", error);
    } finally {
      this.storageSyncInProgress = false;
    }
  }

  async register(
    email: string,
    password: string,
    displayName: string,
  ): Promise<AuthUser> {
    const payload = await postJSON<RawAuthUser>("/api/auth/register", {
      email: email.trim().toLowerCase(),
      password,
      display_name: displayName.trim(),
    });
    return mapUser(payload);
  }

  async login(email: string, password: string): Promise<AuthSession> {
    const payload = await postJSON<TokenResponse>("/api/auth/login", {
      email: email.trim().toLowerCase(),
      password,
    });
    const session = mapResponse(payload);
    this.setSession(session);
    return session;
  }

  async resumeFromRefreshCookie(): Promise<AuthSession | null> {
    try {
      const payload = await postJSON<TokenResponse>("/api/auth/refresh");
      const session = mapResponse(payload);
      this.setSession(session);
      return session;
    } catch (error) {
      if (error instanceof Error) {
        const message = error.message.toLowerCase();
        if (
          message.includes("refresh token required") ||
          message.includes("http 400") ||
          message.includes("http 401") ||
          message.includes("http 403")
        ) {
          return null;
        }
      }
      throw error instanceof Error
        ? error
        : new Error("Failed to resume session from refresh cookie");
    }
  }

  async refresh(): Promise<AuthSession | null> {
    if (!this.session) {
      this.clearSession();
      return null;
    }

    if (hasExpired(this.session.refreshExpiry)) {
      this.clearSession();
      throw new Error("Refresh session expired");
    }

    if (!this.refreshPromise) {
      this.refreshPromise = (async () => {
        try {
          const payload = await postJSON<TokenResponse>("/api/auth/refresh");
          const session = mapResponse(payload);
          this.setSession(session);
          return session;
        } catch (error) {
          this.clearSession();
          throw error instanceof Error
            ? error
            : new Error("Failed to refresh session");
        } finally {
          this.refreshPromise = null;
        }
      })();
    }

    return this.refreshPromise;
  }

  async ensureAccessToken(): Promise<string | null> {
    if (!this.session) {
      return null;
    }

    if (!hasExpired(this.session.accessExpiry, EXPIRY_BUFFER_MS)) {
      return this.session.accessToken;
    }

    try {
      const session = await this.refresh();
      return session?.accessToken ?? null;
    } catch (error) {
      console.warn("[authClient] Failed to refresh access token", error);
      return null;
    }
  }

  async logout(): Promise<void> {
    try {
      await postJSON<void>("/api/auth/logout");
    } catch (error) {
      console.warn("[authClient] Logout request failed", error);
    } finally {
      this.clearSession();
    }
  }

  async startOAuth(provider: OAuthProvider): Promise<{ url: string; state: string }> {
    const payload = await getJSON<OAuthStartResponse>(
      `/api/auth/${provider}/login`,
    );
    const url = typeof payload?.url === "string" ? payload.url.trim() : "";
    const state = typeof payload?.state === "string" ? payload.state.trim() : "";
    if (!url) {
      throw new Error("Missing authorization URL");
    }
    return { url, state };
  }

  async waitForOAuthSession(
    provider: OAuthProvider,
    options: OAuthSessionOptions = {},
  ): Promise<AuthSession> {
    if (typeof window === "undefined") {
      throw new Error("OAuth login is only available in the browser");
    }

    const { popup = null, timeoutMs = 2 * 60 * 1000, pollIntervalMs = 800, signal } =
      options;

    return new Promise<AuthSession>((resolve, reject) => {
      let settled = false;
      let timeoutId: ReturnType<typeof window.setTimeout> | null = null;
      let intervalId: ReturnType<typeof window.setInterval> | null = null;

      const cleanup = () => {
        if (timeoutId) {
          window.clearTimeout(timeoutId);
          timeoutId = null;
        }
        if (intervalId) {
          window.clearInterval(intervalId);
          intervalId = null;
        }
        if (signal) {
          signal.removeEventListener("abort", handleAbort);
        }
      };

      const finalize = (result: { session?: AuthSession | null; error?: Error }) => {
        if (settled) {
          return;
        }
        settled = true;
        cleanup();
        if (popup && !popup.closed) {
          popup.close();
        }
        if (result.session) {
          resolve(result.session);
        } else if (result.error) {
          reject(result.error);
        } else {
          reject(new Error("OAuth login failed"));
        }
      };

      const handleAbort = () => {
        finalize({ error: new Error("OAuth login cancelled") });
      };

      if (signal) {
        if (signal.aborted) {
          handleAbort();
          return;
        }
        signal.addEventListener("abort", handleAbort);
      }

      const poll = async () => {
        if (settled) {
          return;
        }

        if (popup && popup.closed) {
          finalize({ error: new Error("OAuth window closed") });
          return;
        }

        try {
          const session = await this.resumeFromRefreshCookie();
          if (session) {
            finalize({ session });
          }
        } catch (error) {
          const err =
            error instanceof Error
              ? error
              : new Error(`OAuth ${provider} login failed`);
          finalize({ error: err });
        }
      };

      timeoutId = window.setTimeout(() => {
        finalize({ error: new Error("OAuth login timed out") });
      }, timeoutMs);

      intervalId = window.setInterval(() => {
        void poll();
      }, pollIntervalMs);

      void poll();
    });
  }

  async adjustPoints(delta: number): Promise<AuthUser> {
    const token = await this.ensureAccessToken();
    if (!token) {
      throw new Error("Not authenticated");
    }
    const payload = await postJSON<RawAuthUser>(
      "/api/auth/points",
      { delta },
      { headers: { Authorization: `Bearer ${token}` } },
    );
    const user = mapUser(payload);
    this.updateSessionUser(user);
    return user;
  }

  async updateSubscription(tier: string, expiresAt?: string | null): Promise<AuthUser> {
    const token = await this.ensureAccessToken();
    if (!token) {
      throw new Error("Not authenticated");
    }
    const payload = await postJSON<RawAuthUser>(
      "/api/auth/subscription",
      {
        tier,
        ...(expiresAt !== undefined ? { expires_at: expiresAt } : {}),
      },
      { headers: { Authorization: `Bearer ${token}` } },
    );
    const user = mapUser(payload);
    this.updateSessionUser(user);
    return user;
  }

  async listPlans(): Promise<SubscriptionPlan[]> {
    const payload = await getJSON<{ plans: RawSubscriptionPlan[] }>("/api/auth/plans");
    if (!payload?.plans?.length) {
      return [];
    }
    return payload.plans.map(mapSubscriptionPlan);
  }
}

export const authClient = new AuthClient();
