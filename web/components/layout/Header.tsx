"use client";

import Link from "next/link";
import { useMemo, type ReactNode } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { Download, History, LogOut, MoreVertical, Trash2, UserCircle2 } from "lucide-react";

import { EnvironmentStrip } from "@/components/status/EnvironmentStrip";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAuth } from "@/lib/auth/context";
import {
  getLanguageLocale,
  useI18n,
  type TranslationKey,
} from "@/lib/i18n";
import { cn } from "@/lib/utils";

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
    await logout();
    router.replace("/login");
  };

  const accountNode: ReactNode = (() => {
    if (authStatus === "loading") {
      return (
        <Button variant="ghost" disabled className="rounded-full px-4 py-2 text-xs">
          {t("auth.account.loading")}
        </Button>
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
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              className="group inline-flex items-center gap-2 rounded-full border border-border bg-background/80 px-2 py-1 pr-3 text-sm font-semibold"
            >
              <Avatar className="h-9 w-9">
                {user.photoURL ? (
                  <AvatarImage
                    src={user.photoURL}
                    alt={user.displayName || user.email || "User"}
                  />
                ) : null}
                <AvatarFallback>{initials}</AvatarFallback>
              </Avatar>
              <span className="hidden text-left text-xs leading-tight sm:block">
                <span className="block font-semibold text-foreground">
                  {user.displayName || user.email}
                </span>
                <span className="text-[10px] uppercase tracking-[0.2em] text-muted-foreground">
                  {t("auth.account.menuTitle")}
                </span>
              </span>
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-64 rounded-2xl">
            <DropdownMenuLabel className="text-[11px]">
              {t("auth.account.signedInAs", { email: user.email })}
            </DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="flex flex-col items-start gap-1">
              <span className="text-sm font-semibold text-foreground">
                {pointsLabel}
              </span>
              <span className="text-xs text-muted-foreground">
                {subscriptionLabel}
              </span>
              {expiryLabel && (
                <span className="text-xs text-muted-foreground">
                  {expiryLabel}
                </span>
              )}
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem asChild>
              <Link href="/sessions" className="flex items-center gap-2">
                <History className="h-4 w-4" aria-hidden />
                <span>{t("navigation.sessions")}</span>
              </Link>
            </DropdownMenuItem>
            <DropdownMenuItem asChild>
              <Link href="/profile" className="flex items-center gap-2">
                <UserCircle2 className="h-4 w-4" aria-hidden />
                <span>{t("auth.account.menuTitle")}</span>
              </Link>
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="text-destructive focus:text-destructive"
              onClick={handleLogout}
            >
              <LogOut className="mr-2 h-4 w-4" />
              {t("auth.account.logout")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      );
    }

    return (
      <Button asChild variant="outline" size="sm" className="rounded-full">
        <Link href={loginHref}>{t("auth.account.login")}</Link>
      </Button>
    );
  })();

  const hasMenuActions = Boolean(onExport || onDelete);

  return (
    <header
      className={cn(
        "layout-header flex items-center justify-between rounded-3xl border border-border bg-card px-4 py-3",
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
              className="mt-0.5 text-sm text-muted-foreground"
              data-testid="console-header-subtitle"
            >
              {subtitle}
            </p>
          )}
          <div className="mt-2">
            <EnvironmentStrip />
          </div>
        </div>
      </div>

      <div className="flex items-center gap-2">
        {actionsSlot}
        {hasMenuActions && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="h-9 w-9 rounded-full border border-border/70 bg-background/70"
                aria-label={t("header.actions.more")}
              >
                <MoreVertical className="h-4 w-4" aria-hidden />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48 rounded-2xl">
              {onExport && (
                <DropdownMenuItem
                  onClick={() => {
                    onExport();
                  }}
                  className="gap-2"
                >
                  <Download className="h-4 w-4" />
                  {t("header.actions.export")}
                </DropdownMenuItem>
              )}
              {onDelete && (
                <DropdownMenuItem
                  onClick={() => {
                    onDelete();
                  }}
                  className="gap-2 text-destructive focus:text-destructive"
                >
                  <Trash2 className="h-4 w-4" />
                  {t("header.actions.delete")}
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
        {accountNode}
      </div>
    </header>
  );
}
