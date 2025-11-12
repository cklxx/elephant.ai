"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  authClient,
  AuthSession,
  AuthUser,
  SubscriptionPlan,
} from "./client";

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

interface AuthContextValue {
  status: AuthStatus;
  session: AuthSession | null;
  user: AuthUser | null;
  accessToken: string | null;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refresh: () => Promise<AuthSession | null>;
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
        await authClient.ensureAccessToken();
      } finally {
        updateFromClient(authClient.getSession());
      }
    })();

    return () => {
      cancelled = true;
      unsubscribe();
    };
  }, []);

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
      logout,
      refresh,
      adjustPoints,
      updateSubscription,
      listPlans,
    }),
    [
      status,
      session,
      login,
      logout,
      refresh,
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
