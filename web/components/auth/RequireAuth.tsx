"use client";

import { useEffect } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/lib/auth/context";
import { useI18n } from "@/lib/i18n";

function buildNextParam(
  pathname: string | null,
  searchParams: URLSearchParams,
): string {
  const base =
    pathname && pathname.startsWith("/") ? pathname : "/conversation";
  const query = searchParams.toString();
  const combined = query ? `${base}?${query}` : base;
  return combined.startsWith("/") ? combined : "/conversation";
}

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { status } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { t } = useI18n();

  useEffect(() => {
    if (status !== "unauthenticated") {
      return;
    }
    const target = buildNextParam(pathname, searchParams);
    const href = `/login?next=${encodeURIComponent(target)}`;
    router.replace(href);
  }, [status, pathname, searchParams, router]);

  if (status === "loading") {
    return (
      <div className="flex min-h-[calc(100vh-4rem)] items-center justify-center text-sm text-slate-500">
        {t("auth.account.loading")}
      </div>
    );
  }

  if (status !== "authenticated") {
    return null;
  }

  return <>{children}</>;
}
