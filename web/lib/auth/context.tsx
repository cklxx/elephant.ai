"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  authClient,
  AuthSession,
  AuthUser,
  OAuthProvider,
  OAuthSessionOptions,
  SubscriptionPlan,
} from "./client";

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

interface AuthContextValue {
  status: AuthStatus;
  session: AuthSession | null;
  user: AuthUser | null;
  accessToken: string | null;
  login: (email: string, password: string) => Promise<void>;
  register: (
    email: string,
    password: string,
    displayName: string,
  ) => Promise<AuthSession>;
  logout: () => Promise<void>;
  refresh: () => Promise<AuthSession | null>;
  loginWithProvider: (provider: OAuthProvider) => Promise<AuthSession>;
  startOAuth: (provider: OAuthProvider) => Promise<{ url: string; state: string }>;
  awaitOAuthSession: (
    provider: OAuthProvider,
    options?: OAuthSessionOptions,
  ) => Promise<AuthSession>;
  adjustPoints: (delta: number) => Promise<AuthUser>;
  updateSubscription: (
    tier: string,
    expiresAt?: string | null,
  ) => Promise<AuthUser>;
  listPlans: () => Promise<SubscriptionPlan[]>;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<AuthSession | null>(() =>
    authClient.getSession(),
  );
  const [status, setStatus] = useState<AuthStatus>("loading");
  const refreshTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(
    null,
  );

  useEffect(() => {
    let cancelled = false;

    const updateFromClient = (next: AuthSession | null) => {
      if (cancelled) {
        return;
      }
      setSession(next);
      setStatus(next ? "authenticated" : "unauthenticated");
    };

    const unsubscribe = authClient.subscribe(updateFromClient);

    (async () => {
      try {
        const existing = authClient.getSession();
        if (!existing) {
          try {
            await authClient.resumeFromRefreshCookie();
          } catch (error) {
            console.warn("[AuthProvider] Failed to resume session from cookie", error);
          }
          return;
        }
        await authClient.ensureAccessToken();
      } catch (error) {
        console.warn("[AuthProvider] Failed to bootstrap session", error);
      } finally {
        updateFromClient(authClient.getSession());
      }
    })();

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    const handleStorage = (event: StorageEvent) => {
      authClient.handleStorageEvent(event);
    };

    window.addEventListener("storage", handleStorage);
    return () => window.removeEventListener("storage", handleStorage);
  }, []);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }

    if (refreshTimerRef.current) {
      window.clearTimeout(refreshTimerRef.current);
      refreshTimerRef.current = null;
    }

    if (status !== "authenticated" || !session) {
      return;
    }

    const expiryTimestamp = Date.parse(session.accessExpiry);
    if (Number.isNaN(expiryTimestamp) || !Number.isFinite(expiryTimestamp)) {
      return;
    }

    const REFRESH_LEEWAY_MS = 60 * 1000; // 1 minute before expiry
    const delay = Math.max(0, expiryTimestamp - Date.now() - REFRESH_LEEWAY_MS);

    if (delay === 0) {
      void authClient.ensureAccessToken().catch((error) => {
        console.warn(
          "[AuthProvider] Failed to proactively refresh access token",
          error,
        );
      });
      return;
    }

    refreshTimerRef.current = window.setTimeout(() => {
      void authClient.ensureAccessToken().catch((error) => {
        console.warn(
          "[AuthProvider] Failed to proactively refresh access token",
          error,
        );
      });
    }, delay);

    return () => {
      if (refreshTimerRef.current) {
        window.clearTimeout(refreshTimerRef.current);
        refreshTimerRef.current = null;
      }
    };
  }, [session, status]);

  const login = useCallback(async (email: string, password: string) => {
    setStatus("loading");
    try {
      const next = await authClient.login(email, password);
      setSession(next);
      setStatus("authenticated");
    } catch (error) {
      setSession(null);
      setStatus("unauthenticated");
      throw error;
    }
  }, []);

  const register = useCallback(
    async (email: string, password: string, displayName: string) => {
      setStatus("loading");
      try {
        await authClient.register(email, password, displayName);
        const next = await authClient.login(email, password);
        setSession(next);
        setStatus("authenticated");
        return next;
      } catch (error) {
        const existing = authClient.getSession();
        setSession(existing);
        setStatus(existing ? "authenticated" : "unauthenticated");
        throw error;
      }
    },
    [],
  );

  const logout = useCallback(async () => {
    setStatus("loading");
    try {
      await authClient.logout();
    } finally {
      const next = authClient.getSession();
      setSession(next);
      setStatus(next ? "authenticated" : "unauthenticated");
    }
  }, []);

  const refresh = useCallback(async () => {
    const next = await authClient.refresh();
    setSession(next);
    setStatus(next ? "authenticated" : "unauthenticated");
    return next;
  }, []);

  const startOAuth = useCallback(
    (provider: OAuthProvider) => authClient.startOAuth(provider),
    [],
  );

  const awaitOAuthSession = useCallback(
    async (provider: OAuthProvider, options?: OAuthSessionOptions) => {
      const existing = authClient.getSession();
      setStatus("loading");
      try {
        const next = await authClient.waitForOAuthSession(provider, options);
        setSession(next);
        setStatus("authenticated");
        return next;
      } catch (error) {
        const current = authClient.getSession();
        const next = current ?? existing;
        setSession(next);
        setStatus(next ? "authenticated" : "unauthenticated");
        throw error instanceof Error
          ? error
          : new Error("OAuth login failed");
      }
    },
    [],
  );

  const loginWithProvider = useCallback(
    async (provider: OAuthProvider) => {
      if (typeof window === "undefined") {
        throw new Error("OAuth login is only available in the browser");
      }

      try {
        const { url } = await startOAuth(provider);
        const popup = window.open(
          url,
          `alex-console-auth-${provider}`,
          "width=520,height=720,noopener,noreferrer",
        );
        if (!popup) {
          throw new Error("OAuth login popup was blocked");
        }
        return await awaitOAuthSession(provider, { popup });
      } catch (error) {
        throw error instanceof Error
          ? error
          : new Error("OAuth login failed");
      }
    },
    [awaitOAuthSession, startOAuth],
  );

  const adjustPoints = useCallback(
    (delta: number) => authClient.adjustPoints(delta),
    [],
  );

  const updateSubscription = useCallback(
    (tier: string, expiresAt?: string | null) =>
      authClient.updateSubscription(tier, expiresAt),
    [],
  );

  const listPlans = useCallback(
    () => authClient.listPlans(),
    [],
  );

  const value = useMemo<AuthContextValue>(
    () => ({
      status,
      session,
      user: session?.user ?? null,
      accessToken: session?.accessToken ?? null,
      login,
      register,
      logout,
      refresh,
      loginWithProvider,
      startOAuth,
      awaitOAuthSession,
      adjustPoints,
      updateSubscription,
      listPlans,
    }),
    [
      status,
      session,
      login,
      register,
      logout,
      refresh,
      loginWithProvider,
      startOAuth,
      awaitOAuthSession,
      adjustPoints,
      updateSubscription,
      listPlans,
    ],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
}
