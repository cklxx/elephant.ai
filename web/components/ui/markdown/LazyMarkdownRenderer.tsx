"use client";

import type React from "react";
import dynamic from "next/dynamic";
import { MarkdownRenderer } from "./MarkdownRenderer";
import type { MarkdownRendererProps } from "./MarkdownRenderer";

const isTest =
  process.env.NODE_ENV === "test" ||
  process.env.VITEST_WORKER !== undefined;

export const LazyMarkdownRenderer = isTest
  ? (MarkdownRenderer as React.ComponentType<MarkdownRendererProps>)
  : dynamic<MarkdownRendererProps>(
      () =>
        import("./MarkdownRenderer").then((mod) => mod.MarkdownRenderer),
      {
        ssr: false,
        loading: () => (
          <div className="h-24 w-full animate-pulse rounded-xl border border-border/70 bg-muted/40" />
        ),
      },
    );
