"use client";

import { FormEvent, Suspense, useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import Link from "next/link";
import { useAuth } from "@/lib/auth/context";
import { useI18n } from "@/lib/i18n";

function resolveNextTarget(raw: string | null): string {
  if (!raw) {
    return "/conversation";
  }
  if (raw.startsWith("/")) {
    return raw;
  }
  return "/conversation";
}

function LoginPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { status, login } = useAuth();
  const { t } = useI18n();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const nextTarget = useMemo(
    () => resolveNextTarget(searchParams.get("next")),
    [searchParams],
  );

  useEffect(() => {
    if (status === "authenticated") {
      router.replace(nextTarget);
    }
  }, [status, router, nextTarget]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitting(true);
    setError(null);

    try {
      await login(email, password);
      router.replace(nextTarget);
    } catch (err) {
      const message =
        err instanceof Error ? err.message : t("auth.login.genericError");
      setError(message);
      setSubmitting(false);
    }
  };

  return (
    <div className="flex min-h-screen flex-col items-center justify-center bg-slate-950 px-4 py-16">
      <div className="w-full max-w-md space-y-8 rounded-3xl border border-white/10 bg-white/10 p-8 text-white shadow-2xl backdrop-blur">
        <div className="space-y-2 text-center">
          <p className="text-xs font-semibold uppercase tracking-[0.35em] text-sky-200">
            {t("console.brand")}
          </p>
          <h1 className="text-3xl font-semibold">{t("auth.login.title")}</h1>
          <p className="text-sm text-slate-200">{t("auth.login.subtitle")}</p>
        </div>
        <form className="space-y-6" onSubmit={handleSubmit}>
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
              disabled={submitting}
            />
          </div>
          <div className="space-y-2">
            <label
              htmlFor="password"
              className="text-xs font-semibold uppercase tracking-[0.3em] text-slate-200"
            >
              {t("auth.login.passwordLabel")}
            </label>
            <input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              required
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              className="w-full rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm text-white shadow-inner focus:border-sky-400 focus:outline-none focus:ring-2 focus:ring-sky-400/50"
              placeholder="••••••••"
              disabled={submitting}
            />
          </div>

          {error && (
            <div className="rounded-xl border border-rose-400/40 bg-rose-500/20 px-4 py-3 text-sm text-rose-100">
              {t("auth.login.errorPrefix")}{" "}
              <span className="font-medium">{error}</span>
            </div>
          )}

          <button
            type="submit"
            className="w-full rounded-xl bg-sky-500 px-4 py-3 text-sm font-semibold uppercase tracking-[0.25em] text-white shadow-lg shadow-sky-500/30 transition hover:bg-sky-400 disabled:cursor-not-allowed disabled:opacity-70"
            disabled={submitting || status === "authenticated"}
          >
            {submitting ? t("auth.login.pending") : t("auth.login.submit")}
          </button>
        </form>

        <div className="text-center text-xs text-slate-300">
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
