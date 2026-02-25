"use client";

import { cn } from "@/lib/utils";

export function AlexWordmark({
  className,
  title = "Eli",
}: {
  className?: string;
  title?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex whitespace-nowrap font-sans text-base font-medium leading-none",
        className,
      )}
      aria-label={title}
    >
      {title}
    </span>
  );
}
