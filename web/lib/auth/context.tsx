"use client";

import type { ReactNode } from "react";
import { useEffect } from "react";
import { create } from "zustand";
import { useShallow } from "zustand/react/shallow";
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
  initialize: () => void | (() => void);
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, displayName: string) => Promise<AuthSession>;
  logout: () => Promise<void>;
  refresh: () => Promise<AuthSession | null>;
  loginWithProvider: (provider: OAuthProvider) => Promise<AuthSession>;
  startOAuth: (provider: OAuthProvider) => Promise<{ url: string; state: string }>;
  awaitOAuthSession: (provider: OAuthProvider, options?: OAuthSessionOptions) => Promise<AuthSession>;
  adjustPoints: (delta: number) => Promise<AuthUser>;
  updateSubscription: (tier: string, expiresAt?: string | null) => Promise<AuthUser>;
  listPlans: () => Promise<SubscriptionPlan[]>;
}

type AuthState = Omit<AuthContextValue, "user"> & {
  setSessionState: (session: AuthSession | null) => void;
};

let refreshTimeoutId: number | null = null;
let initialized = false;
let teardown: (() => void) | null = null;

function clearRefreshTimer() {
  if (typeof window === "undefined") return;
  if (refreshTimeoutId !== null) {
    window.clearTimeout(refreshTimeoutId);
    refreshTimeoutId = null;
  }
}

function scheduleRefresh(session: AuthSession | null) {
  if (typeof window === "undefined") return;
  clearRefreshTimer();
  if (!session) return;

  const expiryTimestamp = Date.parse(session.accessExpiry);
  if (!Number.isFinite(expiryTimestamp)) {
    return;
  }

  const REFRESH_LEEWAY_MS = 60 * 1000;
  const delay = Math.max(0, expiryTimestamp - Date.now() - REFRESH_LEEWAY_MS);

  if (delay === 0) {
    void authClient.ensureAccessToken();
    return;
  }

  refreshTimeoutId = window.setTimeout(() => {
    void authClient.ensureAccessToken();
  }, delay);
}

function attachStorageListener(): (() => void) | undefined {
  if (typeof window === "undefined") return undefined;

  const handleStorage = (event: StorageEvent) => {
    authClient.handleStorageEvent(event);
  };

  window.addEventListener("storage", handleStorage);
  return () => window.removeEventListener("storage", handleStorage);
}

export const useAuthStore = create<AuthState>((set, get) => ({
  status: "loading",
  session: authClient.getSession(),
  accessToken: authClient.getSession()?.accessToken ?? null,
  initialize: () => {
    if (initialized) return teardown ?? undefined;
    initialized = true;

    const syncState = (next: AuthSession | null) => {
      set({
        session: next,
        status: next ? "authenticated" : "unauthenticated",
        accessToken: next?.accessToken ?? null,
      });
      scheduleRefresh(next);
    };

    const unsubscribe = authClient.subscribe(syncState);
    const detachStorage = attachStorageListener();

    const bootstrap = async () => {
      const existing = authClient.getSession();
      if (!existing) {
        set({ session: null, status: "loading", accessToken: null });
        try {
          await authClient.resumeFromRefreshCookie();
        } catch (error) {
          console.warn("[AuthProvider] Failed to resume session from cookie", error);
        }
        syncState(authClient.getSession());
        return;
      }

      try {
        await authClient.ensureAccessToken();
      } catch (error) {
        console.warn("[AuthProvider] Failed to bootstrap session", error);
      }
      syncState(authClient.getSession());
    };

    void bootstrap();

    teardown = () => {
      unsubscribe();
      detachStorage?.();
      clearRefreshTimer();
      initialized = false;
    };

    return teardown;
  },

  setSessionState: (session) => {
    set({
      session,
      status: session ? "authenticated" : "unauthenticated",
      accessToken: session?.accessToken ?? null,
    });
    scheduleRefresh(session);
  },

  login: async (email: string, password: string) => {
    set({ status: "loading" });
    try {
      const next = await authClient.login(email, password);
      get().setSessionState(next);
    } catch (error) {
      get().setSessionState(null);
      throw error instanceof Error ? error : new Error("Login failed");
    }
  },

  register: async (email: string, password: string, displayName: string) => {
    set({ status: "loading" });
    try {
      await authClient.register(email, password, displayName);
      const next = await authClient.login(email, password);
      get().setSessionState(next);
      return next;
    } catch (error) {
      const existing = authClient.getSession();
      get().setSessionState(existing);
      throw error;
    }
  },

  logout: async () => {
    set({ status: "loading" });
    try {
      await authClient.logout();
    } finally {
      const next = authClient.getSession();
      get().setSessionState(next);
    }
  },

  refresh: async () => {
    const next = await authClient.refresh();
    get().setSessionState(next);
    return next;
  },

  startOAuth: (provider: OAuthProvider) => authClient.startOAuth(provider),

  awaitOAuthSession: async (provider: OAuthProvider, options?: OAuthSessionOptions) => {
    const existing = authClient.getSession();
    set({ status: "loading" });
    try {
      const next = await authClient.waitForOAuthSession(provider, options);
      get().setSessionState(next);
      return next;
    } catch (error) {
      const current = authClient.getSession();
      const next = current ?? existing;
      get().setSessionState(next);
      throw error instanceof Error ? error : new Error("OAuth login failed");
    }
  },

  loginWithProvider: async (provider: OAuthProvider) => {
    if (typeof window === "undefined") {
      throw new Error("OAuth login is only available in the browser");
    }

    try {
      const { url } = await get().startOAuth(provider);
      const popup = window.open(
        url,
        `alex-console-auth-${provider}`,
        "width=520,height=720,noopener,noreferrer",
      );
      if (!popup) {
        throw new Error("OAuth login popup was blocked");
      }
      return await get().awaitOAuthSession(provider, { popup });
    } catch (error) {
      throw error instanceof Error ? error : new Error("OAuth login failed");
    }
  },

  adjustPoints: async (delta: number) => {
    const user = await authClient.adjustPoints(delta);
    const session = authClient.getSession();
    get().setSessionState(session);
    return user;
  },

  updateSubscription: async (tier: string, expiresAt?: string | null) => {
    const user = await authClient.updateSubscription(tier, expiresAt);
    const session = authClient.getSession();
    get().setSessionState(session);
    return user;
  },

  listPlans: () => authClient.listPlans(),
}));

export function initializeAuthStore() {
  return useAuthStore.getState().initialize();
}

export function AuthProvider({ children }: { children: ReactNode }) {
  useEffect(() => {
    return initializeAuthStore();
  }, []);

  return <>{children}</>;
}

export function useAuth(): AuthContextValue {
  return useAuthStore(
    useShallow((state) => ({
      status: state.status,
      session: state.session,
      user: state.session?.user ?? null,
      accessToken: state.accessToken,
      initialize: state.initialize,
      login: state.login,
      register: state.register,
      logout: state.logout,
      refresh: state.refresh,
      loginWithProvider: state.loginWithProvider,
      startOAuth: state.startOAuth,
      awaitOAuthSession: state.awaitOAuthSession,
      adjustPoints: state.adjustPoints,
      updateSubscription: state.updateSubscription,
      listPlans: state.listPlans,
    })),
  );
}
