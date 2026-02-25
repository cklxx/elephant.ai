"use client";

import { ReactNode, useMemo } from "react";

import { Badge } from "@/components/ui/badge";
import { cn } from "@/lib/utils";

export type DebugSurfaceTone = "neutral" | "info" | "success" | "warning" | "error";

type DebugSurfaceMeta = {
  label: string;
  value?: string | number | null;
};

export function DebugSurface({
  title,
  description,
  tone = "neutral",
  meta,
  action,
  className,
  children,
}: {
  title: string;
  description?: string;
  tone?: DebugSurfaceTone;
  meta?: DebugSurfaceMeta[];
  action?: ReactNode;
  className?: string;
  children: ReactNode;
}) {
  const toneClass = {
    neutral: "border-border/60 bg-background/70",
    info: "border-sky-200/80 bg-sky-50/40",
    success: "border-emerald-200/80 bg-emerald-50/40",
    warning: "border-amber-200/80 bg-amber-50/45",
    error: "border-rose-200/80 bg-rose-50/45",
  }[tone];

  const normalizedMeta = (meta ?? []).filter((item) => item.value !== undefined && item.value !== null && item.value !== "");

  return (
    <section className={cn("rounded-lg border px-3 py-2.5", toneClass, className)}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold text-foreground/90">{title}</p>
          {description ? <p className="mt-0.5 text-[11px] text-muted-foreground">{description}</p> : null}
        </div>
        {action ? <div className="shrink-0">{action}</div> : null}
      </div>
      {normalizedMeta.length > 0 ? (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {normalizedMeta.map((item) => (
            <Badge key={`${item.label}-${item.value}`} variant="outline" className="text-[10px]">
              {item.label}: {String(item.value)}
            </Badge>
          ))}
        </div>
      ) : null}
      <div className="mt-2">{children}</div>
    </section>
  );
}

type TextChunk = {
  index: number;
  startLine: number;
  endLine: number;
  chars: number;
  text: string;
};

const DEFAULT_CHUNK_MAX_CHARS = 1800;
const DEFAULT_CHUNK_MAX_LINES = 36;

export function splitTextIntoChunks(
  value: string,
  maxCharsPerChunk: number = DEFAULT_CHUNK_MAX_CHARS,
  maxLinesPerChunk: number = DEFAULT_CHUNK_MAX_LINES,
): TextChunk[] {
  const normalized = value.replace(/\r\n?/g, "\n");
  if (normalized.length === 0) {
    return [{ index: 0, startLine: 1, endLine: 1, chars: 0, text: "" }];
  }

  const lines = normalized.split("\n");
  const chunks: TextChunk[] = [];

  let currentParts: string[] = [];
  let currentChars = 0;
  let currentLines = 0;
  let chunkStartLine = 1;

  const pushCurrent = () => {
    if (currentParts.length === 0) return;
    const text = currentParts.join("");
    chunks.push({
      index: chunks.length,
      startLine: chunkStartLine,
      endLine: chunkStartLine + currentLines - 1,
      chars: text.length,
      text,
    });
    currentParts = [];
    currentChars = 0;
    currentLines = 0;
  };

  for (let lineIndex = 0; lineIndex < lines.length; lineIndex += 1) {
    const hasTrailingBreak = lineIndex < lines.length - 1;
    const lineWithBreak = hasTrailingBreak ? `${lines[lineIndex]}\n` : lines[lineIndex];

    if (lineWithBreak.length > maxCharsPerChunk) {
      pushCurrent();
      let start = 0;
      while (start < lineWithBreak.length) {
        const piece = lineWithBreak.slice(start, start + maxCharsPerChunk);
        chunks.push({
          index: chunks.length,
          startLine: lineIndex + 1,
          endLine: lineIndex + 1,
          chars: piece.length,
          text: piece,
        });
        start += maxCharsPerChunk;
      }
      chunkStartLine = lineIndex + 2;
      continue;
    }

    const overflowCurrent =
      currentParts.length > 0 &&
      (currentChars + lineWithBreak.length > maxCharsPerChunk || currentLines >= maxLinesPerChunk);
    if (overflowCurrent) {
      pushCurrent();
      chunkStartLine = lineIndex + 1;
    }

    currentParts.push(lineWithBreak);
    currentChars += lineWithBreak.length;
    currentLines += 1;
  }

  pushCurrent();

  if (chunks.length === 0) {
    return [{ index: 0, startLine: 1, endLine: 1, chars: normalized.length, text: normalized }];
  }

  return chunks;
}

export function ChunkedTextBlock({
  value,
  maxCharsPerChunk = DEFAULT_CHUNK_MAX_CHARS,
  maxLinesPerChunk = DEFAULT_CHUNK_MAX_LINES,
  emptyLabel = "(empty)",
  className,
}: {
  value: string;
  maxCharsPerChunk?: number;
  maxLinesPerChunk?: number;
  emptyLabel?: string;
  className?: string;
}) {
  const chunks = useMemo(
    () => splitTextIntoChunks(value, maxCharsPerChunk, maxLinesPerChunk),
    [value, maxCharsPerChunk, maxLinesPerChunk],
  );

  if (chunks.length <= 1) {
    return (
      <pre
        className={cn(
          "max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md border border-border/50 bg-background/80 px-3 py-2 font-mono text-[12px] leading-relaxed text-foreground/90",
          className,
        )}
      >
        {chunks[0]?.text || emptyLabel}
      </pre>
    );
  }

  const combined = useMemo(() => chunks.map((c) => c.text).join(""), [chunks]);

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
        <Badge variant="outline" className="text-[10px]">
          {chunks.length} chunks · {value.length} chars · lines 1-{chunks[chunks.length - 1].endLine}
        </Badge>
      </div>
      <pre
        className="max-h-[480px] overflow-auto whitespace-pre-wrap break-words rounded-md border border-border/50 bg-background/80 px-3 py-2 font-mono text-[12px] leading-relaxed text-foreground/90"
      >
        {combined || emptyLabel}
      </pre>
    </div>
  );
}
