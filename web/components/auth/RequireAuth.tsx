"use client";

import { Suspense, useEffect, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { useI18n } from "@/lib/i18n";

function buildNextParam(
  pathname: string | null,
  search: string,
): string {
  const base =
    pathname && pathname.startsWith("/") ? pathname : "/conversation";
  const query = search;
  const combined = query ? `${base}?${query}` : base;
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
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { t } = useI18n();

  const search = searchParams.toString();
  const target = useMemo(() => buildNextParam(pathname, search), [pathname, search]);

  useEffect(() => {
    if (status !== "unauthenticated") {
      return;
    }
    const href = `/login?next=${encodeURIComponent(target)}`;
    router.replace(href);
  }, [status, target, router]);

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
