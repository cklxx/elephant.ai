"use client";

import { cn } from "@/lib/utils";

export function AlexWordmark({
  className,
  title = "alex",
}: {
  className?: string;
  title?: string;
}) {
  return (
    <span
      className={cn(
        "inline-flex whitespace-nowrap font-sans text-xl font-medium leading-none",
        className,
      )}
      aria-label={title}
    >
      alex
    </span>
  );
}
