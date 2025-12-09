"use client";

import type React from "react";
import dynamic from "next/dynamic";
import { MarkdownRenderer } from "@/components/ui/markdown";
import type { MarkdownRendererProps } from "@/components/ui/markdown";

// In tests we want synchronous rendering to satisfy assertions without waiting
// for the dynamic chunk; production still uses the lazy client-only renderer.
const isTest = process.env.NODE_ENV === "test" || process.env.VITEST_WORKER !== undefined;

export const LazyMarkdownRenderer = isTest
  ? (MarkdownRenderer as React.ComponentType<MarkdownRendererProps>)
  : dynamic<MarkdownRendererProps>(
      () => import("@/components/ui/markdown").then((mod) => mod.MarkdownRenderer),
      {
        ssr: false,
        loading: () => (
          <div className="h-24 w-full animate-pulse rounded-xl border border-border/70 bg-muted/40" />
        ),
      },
    );
