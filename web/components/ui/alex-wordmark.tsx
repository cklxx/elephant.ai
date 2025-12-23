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
      viewBox="0 0 128 32"
      className={cn("h-5 w-auto", className)}
      role="img"
      aria-label={title}
    >
      <title>{title}</title>
      <g
        fill="none"
        stroke="currentColor"
        strokeWidth="2.6"
        strokeLinecap="round"
        strokeLinejoin="round"
        transform="skewX(-8)"
      >
        {/* A */}
        <path d="M10 26 L22 6 L34 26" />
        <path d="M16 18 H28" />

        {/* l */}
        <path d="M46 6 V23 C46 27 50 28 52 25" />

        {/* e */}
        <path d="M72 19 H60" />
        <path d="M72 19 C72 13 66 11 62 13 C56 16 56 26 64 26 C68 26 71 24 72 22" />

        {/* x */}
        <path d="M84 12 L98 26" />
        <path d="M98 12 L84 26" />

        {/* flourish */}
        <path d="M102 26 C110 28 118 27 124 24" />
      </g>
    </svg>
  );
}

