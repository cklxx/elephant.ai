import { ReactNode } from "react";

import { cn } from "@/lib/utils";

export function PageShell({
  children,
  className,
  padding = "default",
}: {
  children: ReactNode;
  className?: string;
  padding?: "default" | "none";
}) {
  const basePadding = padding === "none" ? "" : "px-4 py-8 sm:px-6 lg:px-10";

  return <main className={cn(basePadding, className)}>{children}</main>;
}

export function PageContainer({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "mx-auto flex w-full max-w-6xl flex-col gap-8 lg:gap-12",
        className,
      )}
    >
      {children}
    </div>
  );
}

export function SectionBlock({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <section className={cn("flex flex-col gap-4", className)}>{children}</section>;
}

export function SectionHeader({
  overline,
  title,
  description,
  actions,
  titleElement: TitleTag = "h2",
  className,
}: {
  overline?: string;
  title: string;
  description?: string;
  actions?: ReactNode;
  titleElement?: "h1" | "h2" | "h3" | "h4" | "h5" | "h6";
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between",
        className,
      )}
    >
      <div className="flex flex-col gap-2">
        {overline ? (
          <p className="text-xs font-semibold tracking-wide text-muted-foreground">
            {overline}
          </p>
        ) : null}
        <div className="flex flex-col gap-1">
          <TitleTag className="text-2xl font-semibold tracking-tight text-foreground sm:text-3xl">
            {title}
          </TitleTag>
          {description ? (
            <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">
              {description}
            </p>
          ) : null}
        </div>
      </div>
      {actions ? <div className="flex flex-wrap gap-2 sm:justify-end">{actions}</div> : null}
    </div>
  );
}
