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
    <svg
      viewBox="0 0 59 10"
      className={cn("h-5 w-auto font-sans font-semibold", className)}
      role="img"
      aria-label={title}
    >
      <title>{title}</title>
      <text x="0" y="8" fill="currentColor">
        alex
      </text>
    </svg>
  );
}
