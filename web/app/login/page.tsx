"use client";

import clsx from "clsx";
import { Chrome, MessageCircle } from "lucide-react";
import {
  FormEvent,
  Suspense,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import QRCode from "qrcode";

import type { OAuthProvider } from "@/lib/auth/client";
import { useAuth } from "@/lib/auth/context";
import { useI18n } from "@/lib/i18n";

type AuthMode = "login" | "register";

type WeChatStatus =
  | "idle"
  | "initializing"
  | "waiting"
  | "refreshing"
  | "expired"
  | "error";

function resolveNextTarget(raw: string | null): string {
  if (!raw) {
    return "/conversation";
  }
  if (raw.startsWith("/")) {
    return raw;
  }
  return "/conversation";
}

function resolveAuthMode(raw: string | null): AuthMode {
  return raw === "register" ? "register" : "login";
}

function LoginPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const {
    status,
    login,
    register,
    loginWithProvider,
    startOAuth,
    awaitOAuthSession,
  } = useAuth();
  const { t } = useI18n();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [mode, setMode] = useState<AuthMode>(() =>
    resolveAuthMode(searchParams.get("mode")),
  );
  const [oauthPending, setOauthPending] = useState<OAuthProvider | null>(null);
  const [wechatModalOpen, setWeChatModalOpen] = useState(false);
  const [wechatAuthUrl, setWeChatAuthUrl] = useState<string | null>(null);
  const [wechatQrDataUrl, setWeChatQrDataUrl] = useState<string | null>(null);
  const [wechatQrError, setWeChatQrError] = useState<string | null>(null);
  const [wechatGenerating, setWeChatGenerating] = useState(false);
  const [wechatStatus, setWeChatStatus] = useState<WeChatStatus>("idle");
  const wechatAbortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    const nextMode = resolveAuthMode(searchParams.get("mode"));
    setMode((current) => (current === nextMode ? current : nextMode));
  }, [searchParams]);

  const switchMode = useCallback(
    (nextMode: AuthMode) => {
      setMode(nextMode);
      setError(null);
      setSubmitting(false);
      setOauthPending(null);
      if (nextMode === "login") {
        setConfirmPassword("");
      }
      const params = new URLSearchParams(searchParams.toString());
      if (nextMode === "login") {
        params.delete("mode");
      } else {
        params.set("mode", nextMode);
      }
      const nextQuery = params.toString();
      if (nextQuery === searchParams.toString()) {
        return;
      }
      if (nextQuery) {
        router.replace(`?${nextQuery}`, { scroll: false });
      } else if (typeof window !== "undefined") {
        router.replace(window.location.pathname, { scroll: false });
      }
    },
    [router, searchParams],
  );

  const nextTarget = useMemo(
    () => resolveNextTarget(searchParams.get("next")),
    [searchParams],
  );

  useEffect(() => {
    if (status === "authenticated") {
      router.replace(nextTarget);
    }
  }, [status, router, nextTarget]);

  const translateError = (err: unknown, intent: AuthMode | "oauth") => {
    if (!(err instanceof Error)) {
      return t("auth.login.genericError");
    }

    const message = err.message.toLowerCase();

    if (intent === "register") {
      if (message.includes("exists")) {
        return t("auth.register.emailExists");
      }
      if (message.includes("password")) {
        return t("auth.register.passwordInvalid");
      }
      return t("auth.register.genericError");
    }

    if (intent === "oauth") {
      if (message.includes("blocked")) {
        return t("auth.oauth.popupBlocked");
      }
      if (message.includes("timed out")) {
        return t("auth.oauth.timeout");
      }
      if (message.includes("browser")) {
        return t("auth.oauth.unsupported");
      }
      if (message.includes("not configured")) {
        return t("auth.oauth.unavailable");
      }
      return t("auth.oauth.genericError");
    }

    if (message.includes("unauthorized") || message.includes("invalid")) {
      return t("auth.login.invalidCredentials");
    }

    return t("auth.login.genericError");
  };

  const isBusy =
    submitting || oauthPending !== null || status === "loading";

  const resetWeChatState = () => {
    setWeChatModalOpen(false);
    setWeChatAuthUrl(null);
    setWeChatQrDataUrl(null);
    setWeChatQrError(null);
    setWeChatGenerating(false);
    setWeChatStatus("idle");
  };

  const cancelWeChatLogin = () => {
    if (wechatAbortRef.current) {
      wechatAbortRef.current.abort();
      wechatAbortRef.current = null;
    }
    setOauthPending(null);
    resetWeChatState();
  };

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setError(null);
    setSubmitting(true);

    try {
      if (mode === "register") {
        if (password !== confirmPassword) {
          setError(t("auth.register.passwordMismatch"));
          setSubmitting(false);
          return;
        }
        await register(email, password, displayName || email);
      } else {
        await login(email, password);
      }
      router.replace(nextTarget);
    } catch (err) {
      setError(translateError(err, mode));
      setSubmitting(false);
    }
  };

  const handleOAuth = async (provider: OAuthProvider) => {
    setError(null);

    if (provider === "wechat") {
      if (wechatAbortRef.current) {
        wechatAbortRef.current.abort();
        wechatAbortRef.current = null;
      }
      resetWeChatState();
      const controller = new AbortController();
      wechatAbortRef.current = controller;
      setOauthPending("wechat");
      setWeChatModalOpen(true);

      const MAX_ATTEMPTS = 3;
      const REFRESH_DELAY_MS = 1500;
      const SESSION_TIMEOUT_MS = 70_000;
      let succeeded = false;

      try {
        for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt += 1) {
          if (controller.signal.aborted) {
            throw new Error("OAuth login cancelled");
          }

          setWeChatStatus(attempt === 0 ? "initializing" : "refreshing");
          setWeChatQrError(null);
          setWeChatQrDataUrl(null);
          setWeChatAuthUrl(null);
          setWeChatGenerating(true);

          let url: string;
          try {
            ({ url } = await startOAuth("wechat"));
          } catch (err) {
            if (controller.signal.aborted) {
              throw err;
            }
            setWeChatStatus("error");
            setWeChatQrError(translateError(err, "oauth"));
            setWeChatAuthUrl(null);
            setWeChatGenerating(false);
            throw err;
          }

          if (controller.signal.aborted) {
            throw new Error("OAuth login cancelled");
          }

          setWeChatAuthUrl(url);

          try {
            await awaitOAuthSession("wechat", {
              signal: controller.signal,
              timeoutMs: SESSION_TIMEOUT_MS,
              pollIntervalMs: 1000,
            });
            succeeded = true;
            router.replace(nextTarget);
            return;
          } catch (err) {
            if (controller.signal.aborted) {
              throw err;
            }

            const message =
              err instanceof Error ? err.message.toLowerCase() : "";
            const isTimeout = message.includes("timed out");
            const isCancelled =
              message.includes("cancel") || message.includes("closed");

            if (isCancelled) {
              throw err;
            }

            if (attempt === MAX_ATTEMPTS - 1 || !isTimeout) {
              setWeChatStatus("error");
              setWeChatQrError(translateError(err, "oauth"));
              setWeChatAuthUrl(null);
              setWeChatGenerating(false);
              throw err;
            }

            setWeChatStatus("expired");
            setWeChatAuthUrl(null);
            setWeChatGenerating(false);

            await new Promise<void>((resolve) => {
              window.setTimeout(resolve, REFRESH_DELAY_MS);
            });
          }
        }
      } catch (err) {
        if (!controller.signal.aborted) {
          setError(translateError(err, "oauth"));
        }
      } finally {
        if (wechatAbortRef.current === controller) {
          wechatAbortRef.current = null;
        }
        setOauthPending(null);
        if (controller.signal.aborted || succeeded) {
          resetWeChatState();
        }
      }
      return;
    }

    setOauthPending(provider);
    try {
      await loginWithProvider(provider);
      router.replace(nextTarget);
    } catch (err) {
      setError(translateError(err, "oauth"));
    } finally {
      setOauthPending(null);
    }
  };

  useEffect(() => {
    let cancelled = false;

    if (!wechatModalOpen) {
      setWeChatQrDataUrl(null);
      setWeChatQrError(null);
      setWeChatGenerating(false);
      return;
    }

    if (!wechatAuthUrl) {
      setWeChatQrDataUrl(null);
      setWeChatQrError(null);
      return;
    }

    setWeChatGenerating(true);
    setWeChatQrDataUrl(null);
    setWeChatQrError(null);

    QRCode.toDataURL(wechatAuthUrl, {
      margin: 1,
      scale: 6,
      width: 240,
      color: {
        dark: "#0f172a",
        light: "#ffffff",
      },
    })
      .then((dataUrl) => {
        if (!cancelled) {
          setWeChatQrDataUrl(dataUrl);
          setWeChatStatus("waiting");
        }
      })
      .catch((error) => {
        console.warn("[LoginPage] Failed to render WeChat QR code", error);
        if (!cancelled) {
          setWeChatQrError(t("auth.oauth.wechat.qrError"));
          setWeChatStatus("error");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setWeChatGenerating(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [t, wechatAuthUrl, wechatModalOpen]);

  useEffect(() => {
    return () => {
      if (wechatAbortRef.current) {
        wechatAbortRef.current.abort();
        wechatAbortRef.current = null;
      }
    };
  }, []);

  const wechatStatusLabel = useMemo(() => {
    switch (wechatStatus) {
      case "initializing":
        return t("auth.oauth.wechat.status.initializing");
      case "waiting":
        return t("auth.oauth.wechat.status.waiting");
      case "refreshing":
        return t("auth.oauth.wechat.status.refreshing");
      case "expired":
        return t("auth.oauth.wechat.status.expired");
      case "error":
        return t("auth.oauth.wechat.status.error");
      default:
        return null;
    }
  }, [t, wechatStatus]);

  return (
    <>
      <div className="flex min-h-screen flex-col items-center justify-center bg-slate-950 px-4 py-16">
        <div className="w-full max-w-md space-y-8 rounded-3xl border border-white/10 bg-white/10 p-8 text-white shadow-2xl backdrop-blur">
          <div className="space-y-6 text-center">
            <p className="text-xs font-semibold uppercase tracking-[0.35em] text-sky-200">
              {t("console.brand")}
            </p>
          <div className="space-y-2">
            <h1 className="text-3xl font-semibold">
              {mode === "login"
                ? t("auth.login.title")
                : t("auth.register.title")}
            </h1>
            <p className="text-sm text-slate-200">
              {mode === "login"
                ? t("auth.login.subtitle")
                : t("auth.register.subtitle")}
            </p>
          </div>
          <div className="inline-flex rounded-full border border-white/10 bg-white/10 p-1">
            {(["login", "register"] as AuthMode[]).map((item) => (
              <button
                key={item}
                type="button"
                className={clsx(
                  "relative flex-1 rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.3em] transition",
                  item === mode
                    ? "bg-sky-500 text-white shadow-lg shadow-sky-500/30"
                    : "text-slate-200 hover:text-white",
                )}
                onClick={() => {
                  switchMode(item);
                }}
                aria-pressed={item === mode}
              >
                {item === "login"
                  ? t("auth.login.mode.signIn")
                  : t("auth.login.mode.register")}
              </button>
            ))}
          </div>
        </div>

        <div className="space-y-6">
          <div className="grid gap-3">
            <button
              type="button"
              onClick={() => handleOAuth("google")}
              className="flex w-full items-center justify-center gap-2 rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-white transition hover:border-white/40 hover:bg-white/20 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={isBusy}
            >
              <Chrome className="h-4 w-4" aria-hidden="true" />
              {oauthPending === "google"
                ? t("auth.oauth.pending")
                : t("auth.oauth.google")}
            </button>
            <button
              type="button"
              onClick={() => handleOAuth("wechat")}
              className="flex w-full items-center justify-center gap-2 rounded-xl border border-emerald-400/40 bg-emerald-500/20 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-emerald-100 transition hover:border-emerald-300 hover:bg-emerald-500/30 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={isBusy}
            >
              <MessageCircle className="h-4 w-4" aria-hidden="true" />
              {oauthPending === "wechat"
                ? t("auth.oauth.pending")
                : t("auth.oauth.wechat")}
            </button>
          </div>

          <div className="flex items-center gap-3 text-xs font-semibold uppercase tracking-[0.3em] text-slate-300">
            <span className="h-px flex-1 bg-white/10" aria-hidden="true" />
            <span>
              {mode === "login"
                ? t("auth.login.emailDivider")
                : t("auth.register.emailDivider")}
            </span>
            <span className="h-px flex-1 bg-white/10" aria-hidden="true" />
          </div>

          <form className="space-y-5" onSubmit={handleSubmit}>
            {mode === "register" && (
              <div className="space-y-2">
                <label
                  htmlFor="displayName"
                  className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-200"
                >
                  {t("auth.register.displayNameLabel")}
                </label>
                <input
                  id="displayName"
                  name="displayName"
                  type="text"
                  autoComplete="name"
                  required
                  value={displayName}
                  onChange={(event) => setDisplayName(event.target.value)}
                  className="w-full rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm text-white shadow-inner focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/50"
                  placeholder={t("auth.register.displayNamePlaceholder")}
                  disabled={isBusy}
                />
              </div>
            )}

            <div className="space-y-2">
              <label
                htmlFor="email"
                className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-200"
              >
                {t("auth.login.emailLabel")}
              </label>
              <input
                id="email"
                name="email"
                type="email"
                autoComplete="email"
                required
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                className="w-full rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm text-white shadow-inner focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/50"
                placeholder="name@example.com"
                disabled={isBusy}
              />
            </div>

            <div className="space-y-2">
              <label
                htmlFor="password"
                className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-200"
              >
                {mode === "login"
                  ? t("auth.login.passwordLabel")
                  : t("auth.register.passwordLabel")}
              </label>
              <input
                id="password"
                name="password"
                type="password"
                autoComplete={mode === "login" ? "current-password" : "new-password"}
                required
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                className="w-full rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm text-white shadow-inner focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/50"
                placeholder="••••••••"
                disabled={isBusy}
              />
            </div>

            {mode === "register" && (
              <div className="space-y-2">
                <label
                  htmlFor="confirmPassword"
                  className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-200"
                >
                  {t("auth.register.confirmPasswordLabel")}
                </label>
                <input
                  id="confirmPassword"
                  name="confirmPassword"
                  type="password"
                  autoComplete="new-password"
                  required
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  className="w-full rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm text-white shadow-inner focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/50"
                  placeholder="••••••••"
                  disabled={isBusy}
                />
              </div>
            )}

            {error && (
              <div className="rounded-xl border border-rose-400/40 bg-rose-500/20 px-4 py-3 text-sm text-rose-100">
                {t("auth.login.errorPrefix")} {" "}
                <span className="font-medium">{error}</span>
              </div>
            )}

            <button
              type="submit"
              className="w-full rounded-xl bg-sky-500 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-white shadow-lg shadow-sky-500/30 transition hover:bg-sky-400 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={isBusy || status === "authenticated"}
            >
              {submitting
                ? mode === "login"
                  ? t("auth.login.pending")
                  : t("auth.register.pending")
                : mode === "login"
                ? t("auth.login.submit")
                : t("auth.register.submit")}
            </button>
          </form>
        </div>

        <div className="text-center text-xs text-slate-300">
          {mode === "login" ? (
            <>
              <span>{t("auth.login.noAccount")}</span>{" "}
              <button
                type="button"
                onClick={() => {
                  switchMode("register");
                }}
                className="font-semibold text-sky-200 hover:text-sky-100"
              >
                {t("auth.login.switchToRegister")}
              </button>
            </>
          ) : (
            <>
              <span>{t("auth.register.haveAccount")}</span>{" "}
              <button
                type="button"
                onClick={() => {
                  switchMode("login");
                }}
                className="font-semibold text-sky-200 hover:text-sky-100"
              >
                {t("auth.register.switchToLogin")}
              </button>
            </>
          )}
          <div className="mt-2">
            <span>{t("auth.login.needHelp")}</span>{" "}
            <Link
              href="https://docs.alex-console.invalid"
              className="font-semibold text-sky-200 hover:text-sky-100"
            >
              {t("auth.login.contactAdmin")}
            </Link>
          </div>
        </div>
      </div>
      </div>

      {wechatModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4 py-8">
          <div className="w-full max-w-sm space-y-6 rounded-3xl border border-white/10 bg-slate-900/95 p-6 text-white shadow-2xl">
            <div className="space-y-2 text-center">
              <h2 className="text-xl font-semibold">
                {t("auth.oauth.wechat.title")}
              </h2>
              <p className="text-sm text-slate-300">
                {t("auth.oauth.wechat.subtitle")}
              </p>
            </div>
            <div className="flex justify-center">
              {wechatQrDataUrl ? (
                <Image
                  src={wechatQrDataUrl}
                  alt={t("auth.oauth.wechat.alt")}
                  width={192}
                  height={192}
                  unoptimized
                  className="h-48 w-48 rounded-2xl border border-white/10 bg-white p-3 shadow-inner"
                />
              ) : wechatGenerating ? (
                <div className="flex h-48 w-48 items-center justify-center rounded-2xl border border-white/10 bg-white/5 px-4 text-center text-sm text-slate-200">
                  {t("auth.oauth.wechat.generating")}
                </div>
              ) : wechatStatus === "error" ? (
                <div className="flex h-48 w-48 items-center justify-center rounded-2xl border border-rose-400/40 bg-rose-500/15 px-4 text-center text-sm text-rose-100">
                  {wechatQrError ?? t("auth.oauth.wechat.qrError")}
                </div>
              ) : (
                <div className="flex h-48 w-48 items-center justify-center rounded-2xl border border-white/10 bg-white/5 px-4 text-center text-sm text-slate-200">
                  {wechatStatus === "expired"
                    ? t("auth.oauth.wechat.status.expired")
                    : t("auth.oauth.wechat.generating")}
                </div>
              )}
            </div>
            {wechatStatusLabel && (
              <p className="text-center text-xs text-slate-300">
                {wechatStatusLabel}
              </p>
            )}
            <button
              type="button"
              onClick={cancelWeChatLogin}
              className="w-full rounded-xl border border-white/10 bg-white/5 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-slate-100 transition hover:border-white/30 hover:bg-white/10"
            >
              {t("auth.oauth.wechat.cancel")}
            </button>
          </div>
        </div>
      )}
    </>
  );
}

function LoginPageFallback() {
  const { t } = useI18n();

  return (
    <div className="flex min-h-screen items-center justify-center bg-slate-950 px-4 py-16 text-white">
      {t("app.loading")}
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense fallback={<LoginPageFallback />}>
      <LoginPageContent />
    </Suspense>
  );
}
