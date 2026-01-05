"use client";

import { Suspense, useEffect, useRef } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { useI18n } from "@/lib/i18n";

function buildNextParam(
  pathname: string | null,
  search: string,
): string {
  const base =
    pathname && pathname.startsWith("/") ? pathname : "/conversation";
  const combined = search ? `${base}?${search}` : base;
  return combined.startsWith("/") ? combined : "/conversation";
}

function RequireAuthFallback() {
  const { t } = useI18n();

  return (
    <div className="flex min-h-[calc(100vh-4rem)] items-center justify-center text-sm text-slate-500">
      {t("auth.account.loading")}
    </div>
  );
}

function RequireAuthContent({
  children,
}: {
  children: React.ReactNode;
}) {
  const { status } = useAuth();
  const router = useRouter();
  const { t } = useI18n();
  const lastStatusRef = useRef<
    "loading" | "authenticated" | "unauthenticated" | null
  >(null);

  useEffect(() => {
    const previousStatus = lastStatusRef.current;
    lastStatusRef.current = status;

    if (status !== "unauthenticated" || previousStatus === "unauthenticated") {
      return;
    }

    const pathname =
      typeof window !== "undefined" ? window.location.pathname : null;
    const search =
      typeof window !== "undefined"
        ? window.location.search.replace(/^\?/, "")
        : "";
    const target = buildNextParam(pathname, search);
    const href = `/login?next=${encodeURIComponent(target)}`;
    router.replace(href);
  }, [status, router]);

  if (status === "loading") {
    return <RequireAuthFallback />;
  }

  if (status !== "authenticated") {
    return null;
  }

  return <>{children}</>;
}

export function RequireAuth({ children }: { children: React.ReactNode }) {
  return (
    <Suspense fallback={<RequireAuthFallback />}>
      <RequireAuthContent>{children}</RequireAuthContent>
    </Suspense>
  );
}
