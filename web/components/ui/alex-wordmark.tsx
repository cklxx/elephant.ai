"use client";

import { cn } from "@/lib/utils";

export function AlexWordmark({
  className,
  title = "Alex",
}: {
  className?: string;
  title?: string;
}) {
  return (
    <svg
      viewBox="0 0 110 32"
      className={cn("h-5 w-auto", className)}
      role="img"
      aria-label={title}
    >
      <title>{title}</title>
      <text
        x="0"
        y="22"
        className="fill-current"
        style={{
          fontFamily:
            "var(--font-sans), ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif",
          fontWeight: 700,
          fontStyle: "italic",
          letterSpacing: "-0.6px",
        }}
      >
        Alex
      </text>
      <path
        d="M64 25 C 76 29 92 28 108 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2.2"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity="0.9"
      />
    </svg>
  );
}
