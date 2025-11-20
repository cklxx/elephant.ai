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
      <div className="flex min-h-screen w-full items-center justify-center bg-transparent px-4 py-10 text-[hsl(var(--foreground))]">
        <div className="flex w-full max-w-6xl flex-col items-center gap-10 lg:flex-row lg:items-center">
          <div className="relative w-full max-w-2xl space-y-8 rounded-[40px] bg-white/5 p-8 backdrop-blur">
            <div className="relative space-y-8 text-center">
              <p className="text-xs font-semibold uppercase tracking-[0.35em] text-gray-500">
                {t("console.brand")}
              </p>
              <div className="space-y-2">
                <h1 className="text-4xl font-semibold leading-tight">
                  {mode === "login"
                    ? t("auth.login.title")
                    : t("auth.register.title")}
                </h1>
                <p className="text-base text-gray-600">
                  {mode === "login"
                    ? t("auth.login.subtitle")
                    : t("auth.register.subtitle")}
                </p>
              </div>
              <div className="mx-auto w-full max-w-xl rounded-[28px] bg-white/5 p-6 backdrop-blur">
                <div className="grid grid-cols-5 gap-3 opacity-90">
                  {Array.from({ length: 10 }).map((_, index) => (
                    <div
                      key={index}
                      className={clsx(
                        "aspect-square rounded-2xl bg-white/60",
                        index % 3 === 0 && "bg-white/20",
                      )}
                    />
                  ))}
                </div>
                <div className="mt-6 space-y-3">
                  <div className="h-3 rounded-full bg-white/70" />
                  <div className="h-3 w-3/4 rounded-full bg-white/70" />
                  <div className="flex gap-3">
                    <div className="h-16 flex-1 rounded-[24px] bg-white/30" />
                    <div className="h-16 flex-1 rounded-[24px] bg-white/15" />
                  </div>
                </div>
              </div>
              <div className="inline-flex rounded-[999px] bg-white/10 p-1 backdrop-blur">
                {(["login", "register"] as AuthMode[]).map((item) => (
                  <button
                    key={item}
                    type="button"
                    className={clsx(
                      "relative flex-1 rounded-[999px] px-4 py-2 text-xs font-semibold uppercase tracking-[0.3em] transition-transform duration-200 hover:-translate-y-0.5 focus:outline-none focus-visible:ring-2 focus-visible:ring-[hsl(var(--foreground))]",
                      item === mode
                        ? "bg-[hsl(var(--foreground))] text-[hsl(var(--background))]"
                        : "bg-white/10 text-[hsl(var(--foreground))]",
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
          </div>

          <div className="relative w-full max-w-2xl space-y-6">
            <div className="grid w-full max-w-xl gap-3">
              <button
                type="button"
                onClick={() => handleOAuth("google")}
                className="flex w-full items-center justify-center gap-2 rounded-[24px] bg-white/10 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-[hsl(var(--foreground))] transition-transform hover:-translate-y-0.5 disabled:translate-y-0 disabled:opacity-60 backdrop-blur"
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
                className="flex w-full items-center justify-center gap-2 rounded-[24px] bg-emerald-500/15 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-emerald-900 transition-transform hover:-translate-y-0.5 disabled:translate-y-0 disabled:opacity-60 backdrop-blur"
                disabled={isBusy}
              >
                <MessageCircle className="h-4 w-4" aria-hidden="true" />
                {oauthPending === "wechat"
                  ? t("auth.oauth.pending")
                  : t("auth.oauth.wechat")}
              </button>
            </div>

            <div className="flex items-center gap-3 text-xs font-semibold uppercase tracking-[0.3em] text-gray-500">
              <span className="h-px flex-1 bg-white/20" aria-hidden="true" />
              <span>
                {mode === "login"
                  ? t("auth.login.emailDivider")
                  : t("auth.register.emailDivider")}
              </span>
              <span className="h-px flex-1 bg-white/20" aria-hidden="true" />
            </div>

            <form
              className="w-full max-w-xl space-y-5 rounded-[32px] bg-white/8 p-6 backdrop-blur"
              onSubmit={handleSubmit}
            >
              {mode === "register" && (
                <div className="space-y-2">
                  <label
                    htmlFor="displayName"
                    className="text-xs font-semibold uppercase tracking-[0.3em] text-muted-foreground"
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
                  className="w-full rounded-[20px] border border-input bg-background/90 px-4 py-3 text-sm text-[hsl(var(--foreground))] focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring disabled:bg-background/40"
                  placeholder={t("auth.register.displayNamePlaceholder")}
                  disabled={isBusy}
                />
              </div>
            )}

            <div className="space-y-2">
              <label
                htmlFor="email"
                className="text-xs font-semibold uppercase tracking-[0.3em] text-muted-foreground"
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
                className="w-full rounded-[20px] border border-input bg-background/90 px-4 py-3 text-sm text-[hsl(var(--foreground))] focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring disabled:bg-background/40"
                placeholder="name@example.com"
                disabled={isBusy}
              />
            </div>

            <div className="space-y-2">
              <label
                htmlFor="password"
                className="text-xs font-semibold uppercase tracking-[0.3em] text-muted-foreground"
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
                className="w-full rounded-[20px] border border-input bg-background/90 px-4 py-3 text-sm text-[hsl(var(--foreground))] focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring disabled:bg-background/40"
                placeholder="••••••••"
                disabled={isBusy}
              />
            </div>

            {mode === "register" && (
              <div className="space-y-2">
                <label
                  htmlFor="confirmPassword"
                  className="text-xs font-semibold uppercase tracking-[0.3em] text-muted-foreground"
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
                  className="w-full rounded-[20px] border border-input bg-background/90 px-4 py-3 text-sm text-[hsl(var(--foreground))] focus:border-ring focus:outline-none focus:ring-2 focus:ring-ring disabled:bg-background/40"
                  placeholder="••••••••"
                  disabled={isBusy}
                />
              </div>
            )}

            {error && (
              <div className="rounded-[20px] bg-rose-500/15 px-4 py-3 text-sm font-semibold text-rose-700">
                {t("auth.login.errorPrefix")} {" "}
                <span className="font-bold">{error}</span>
              </div>
            )}

            <button
              type="submit"
              className="w-full rounded-[28px] bg-[hsl(var(--foreground))] px-4 py-3 text-sm font-semibold uppercase tracking-[0.35em] text-[hsl(var(--background))] transition-transform hover:-translate-y-0.5 disabled:translate-y-0 disabled:opacity-60"
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

        <div className="text-center text-sm text-gray-600">
          {mode === "login" ? (
            <>
              <span>{t("auth.login.noAccount")}</span>{" "}
              <button
                type="button"
                onClick={() => {
                  switchMode("register");
                }}
                className="font-semibold text-[hsl(var(--foreground))] underline-offset-4 hover:underline"
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
                className="font-semibold text-[hsl(var(--foreground))] underline-offset-4 hover:underline"
              >
                {t("auth.register.switchToLogin")}
              </button>
            </>
          )}
          <div className="mt-2">
            <span>{t("auth.login.needHelp")}</span>{" "}
            <Link
              href="https://docs.alex-console.invalid"
              className="font-semibold text-[hsl(var(--foreground))] underline-offset-4 hover:underline"
            >
              {t("auth.login.contactAdmin")}
            </Link>
          </div>
        </div>
      </div>
      </div>

      {wechatModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 px-4 py-8">
          <div className="w-full max-w-sm space-y-6 rounded-[32px] bg-white/10 p-6 text-[hsl(var(--foreground))] backdrop-blur">
            <div className="space-y-2 text-center">
              <h2 className="text-xl font-semibold">{t("auth.oauth.wechat.title")}</h2>
              <p className="text-sm text-gray-600">{t("auth.oauth.wechat.subtitle")}</p>
            </div>
            <div className="flex justify-center">
              {wechatQrDataUrl ? (
                <Image
                  src={wechatQrDataUrl}
                  alt={t("auth.oauth.wechat.alt")}
                  width={192}
                  height={192}
                  unoptimized
                  className="h-48 w-48 rounded-[24px] bg-white/80 p-3 shadow-none"
                />
              ) : wechatGenerating ? (
                <div className="flex h-48 w-48 items-center justify-center rounded-[24px] bg-white/50 px-4 text-center text-sm text-gray-700">
                  {t("auth.oauth.wechat.generating")}
                </div>
              ) : wechatStatus === "error" ? (
                <div className="flex h-48 w-48 items-center justify-center rounded-[24px] bg-rose-500/15 px-4 text-center text-sm text-rose-700">
                  {wechatQrError ?? t("auth.oauth.wechat.qrError")}
                </div>
              ) : (
                <div className="flex h-48 w-48 items-center justify-center rounded-[24px] bg-white/60 px-4 text-center text-sm text-gray-700">
                  {wechatStatus === "expired"
                    ? t("auth.oauth.wechat.status.expired")
                    : t("auth.oauth.wechat.generating")}
                </div>
              )}
            </div>
            {wechatStatusLabel && (
              <p className="text-center text-xs text-gray-600">{wechatStatusLabel}</p>
            )}
            <button
              type="button"
              onClick={cancelWeChatLogin}
              className="w-full rounded-[24px] bg-white/15 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-[hsl(var(--foreground))] transition-transform hover:-translate-y-0.5"
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
    <div className="flex min-h-screen items-center justify-center bg-[hsl(var(--background))] px-4 py-16 text-[hsl(var(--foreground))]">
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
