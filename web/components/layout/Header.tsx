"use client";

import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { History, MoreVertical, Download, Trash2, UserCircle2 } from "lucide-react";
import {
  getLanguageLocale,
  useI18n,
  type TranslationKey,
} from "@/lib/i18n";
import { cn } from "@/lib/utils";
import { EnvironmentStrip } from "@/components/status/EnvironmentStrip";
import { useAuth } from "@/lib/auth/context";

interface HeaderProps {
  title?: string;
  subtitle?: string;
  onExport?: () => void;
  onDelete?: () => void;
  className?: string;
  leadingSlot?: ReactNode;
  actionsSlot?: ReactNode;
}

export function Header({
  title,
  subtitle,
  onExport,
  onDelete,
  className,
  leadingSlot,
  actionsSlot,
}: HeaderProps) {
  const { t, language } = useI18n();
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const { status: authStatus, user, logout } = useAuth();
  const [showMenu, setShowMenu] = useState(false);
  const [showAccountMenu, setShowAccountMenu] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const accountMenuRef = useRef<HTMLDivElement>(null);
  const hasMenuActions = Boolean(onExport || onDelete);

  const locale = useMemo(() => getLanguageLocale(language), [language]);

  const currencyFormatter = useMemo(
    () =>
      new Intl.NumberFormat(locale, {
        style: "currency",
        currency: "USD",
        maximumFractionDigits: 0,
      }),
    [locale],
  );

  const dateFormatter = useMemo(
    () =>
      new Intl.DateTimeFormat(locale, {
        dateStyle: "medium",
      }),
    [locale],
  );

  useEffect(() => {
    if (!showMenu) {
      return;
    }

    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setShowMenu(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [showMenu]);

  useEffect(() => {
    if (!showAccountMenu) {
      return;
    }

    const handleClickOutside = (event: MouseEvent) => {
      if (
        accountMenuRef.current &&
        !accountMenuRef.current.contains(event.target as Node)
      ) {
        setShowAccountMenu(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [showAccountMenu]);

  const nextTarget = useMemo(() => {
    const base = pathname ?? "/conversation";
    const query = searchParams.toString();
    const combined = query ? `${base}?${query}` : base;
    return combined.startsWith("/") ? combined : "/conversation";
  }, [pathname, searchParams]);

  const loginHref = useMemo(
    () => `/login?next=${encodeURIComponent(nextTarget)}`,
    [nextTarget],
  );

  const handleLogout = async () => {
    setShowAccountMenu(false);
    await logout();
    router.replace("/login");
  };

  const accountNode: ReactNode = (() => {
    if (authStatus === "loading") {
      return (
        <div className="rounded-full bg-white/10 px-4 py-2 text-xs font-semibold uppercase tracking-[0.2em] text-gray-400 backdrop-blur">
          {t("auth.account.loading")}
        </div>
      );
    }

    if (authStatus === "authenticated" && user) {
      const initials = (user.displayName || user.email || "?")
        .trim()
        .charAt(0)
        .toUpperCase();
      const tierKey = (
        `auth.subscription.tier.${user.subscription.tier}` as TranslationKey
      );
      const tierLabel = t(tierKey);
      const subscriptionLabel = user.subscription.isPaid
        ? t("auth.account.subscriptionPaid", {
            tier: tierLabel,
            price: currencyFormatter.format(
              user.subscription.monthlyPriceCents / 100,
            ),
          })
        : t("auth.account.subscriptionFree", { tier: tierLabel });
      const expiryLabel =
        user.subscription.isPaid && user.subscription.expiresAt
          ? t("auth.account.subscriptionExpires", {
              date: dateFormatter.format(
                new Date(user.subscription.expiresAt),
              ),
            })
          : null;
      const pointsLabel = t("auth.account.points", {
        count: user.pointsBalance,
      });
      return (
        <div className="relative" ref={accountMenuRef}>
          <button
            type="button"
            onClick={() => setShowAccountMenu((prev) => !prev)}
            className="flex items-center gap-2 rounded-full bg-white/10 px-3 py-1.5 text-sm font-semibold text-foreground backdrop-blur transition hover:bg-white/20"
            aria-haspopup="true"
            aria-expanded={showAccountMenu}
          >
            <span className="flex h-7 w-7 items-center justify-center rounded-full bg-slate-900 text-xs font-semibold text-white">
              {initials}
            </span>
            <span className="hidden text-left text-xs leading-tight sm:block">
              <span className="block font-semibold text-slate-800">
                {user.displayName || user.email}
              </span>
              <span className="text-[10px] uppercase tracking-[0.2em] text-slate-400">
                {t("auth.account.menuTitle")}
              </span>
            </span>
          </button>
            {showAccountMenu && (
            <div className="absolute right-0 top-full z-50 mt-2 w-60 rounded-2xl bg-white/10 text-foreground shadow-none backdrop-blur">
              <div className="px-4 py-3">
                <p className="text-xs font-semibold uppercase tracking-[0.2em] text-gray-400">
                  {t("auth.account.signedInAs", { email: user.email })}
                </p>
              </div>
              <div className="px-4 py-3 text-xs text-gray-200/90">
                <p className="text-sm font-semibold text-foreground">
                  {pointsLabel}
                </p>
                <p className="mt-1 text-gray-300">{subscriptionLabel}</p>
                {expiryLabel && (
                  <p className="mt-1 text-gray-400">{expiryLabel}</p>
                )}
              </div>
              <Link
                href="/sessions"
                onClick={() => setShowAccountMenu(false)}
                className="flex w-full items-center gap-3 px-4 py-3 text-left text-sm font-semibold text-foreground transition hover:bg-white/10"
              >
                <History className="h-4 w-4" aria-hidden />
                <span>{t("navigation.sessions")}</span>
              </Link>
              <button
                type="button"
                onClick={handleLogout}
                className="flex w-full items-center gap-3 px-4 py-3 text-left text-sm font-semibold text-foreground transition hover:bg-white/10"
              >
                <UserCircle2 className="h-4 w-4" />
                <span>{t("auth.account.logout")}</span>
              </button>
            </div>
          )}
        </div>
      );
    }

    return (
      <Link
        href={loginHref}
        className="console-button console-button-secondary inline-flex items-center justify-center !px-3 !py-2 text-xs font-semibold uppercase tracking-[0.25em]"
      >
        {t("auth.account.login")}
      </Link>
    );
  })();

  return (
    <header
      className={cn(
        "layout-header flex items-center justify-between px-6 py-4",
        className,
      )}
    >
      <div className="flex flex-1 items-center gap-4">
        {leadingSlot && <div className="flex items-center">{leadingSlot}</div>}
        <div className="flex-1">
          {title && (
            <h1
              className="text-lg font-semibold text-foreground"
              data-testid="console-header-title"
            >
              {title}
            </h1>
          )}
          {subtitle && (
            <p
              className="mt-0.5 text-sm text-gray-600"
              data-testid="console-header-subtitle"
            >
              {subtitle}
            </p>
          )}
          <EnvironmentStrip />
        </div>
      </div>

      <div className="flex items-center gap-2">
        {actionsSlot}
        {hasMenuActions && (
          <div className="relative" ref={menuRef}>
            <button
              type="button"
              onClick={() => setShowMenu((prev) => !prev)}
              className="console-button console-button-ghost inline-flex items-center justify-center !px-2 !py-2"
              aria-haspopup="true"
              aria-expanded={showMenu}
              aria-label={t("header.actions.more")}
            >
              <MoreVertical className="h-4 w-4" aria-hidden />
            </button>

            {showMenu && (
              <div className="absolute right-0 top-full z-50 mt-2 w-48 rounded-2xl bg-white/10 p-1 shadow-none backdrop-blur">
                {onExport && (
                  <button
                    onClick={() => {
                      onExport();
                      setShowMenu(false);
                    }}
                    className="flex w-full items-center gap-3 rounded-xl px-4 py-2 text-left text-sm uppercase tracking-[0.12em] text-foreground transition hover:bg-white/10"
                  >
                    <Download className="h-4 w-4" />
                    <span>{t("header.actions.export")}</span>
                  </button>
                )}
                {onDelete && (
                  <button
                    onClick={() => {
                      onDelete();
                      setShowMenu(false);
                    }}
                    className="flex w-full items-center gap-3 rounded-xl px-4 py-2 text-left text-sm uppercase tracking-[0.12em] text-foreground transition hover:bg-white/10"
                  >
                    <Trash2 className="h-4 w-4" />
                    <span>{t("header.actions.delete")}</span>
                  </button>
                )}
              </div>
            )}
          </div>
        )}
        {accountNode}
      </div>
    </header>
  );
}
