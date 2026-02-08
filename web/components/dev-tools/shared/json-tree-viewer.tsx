"use client";

import React from "react";

function renderScalar(value: unknown) {
  if (value === null) {
    return <span className="text-rose-600">null</span>;
  }
  if (typeof value === "string") {
    return <span className="break-all text-emerald-700">&quot;{value}&quot;</span>;
  }
  if (typeof value === "number") {
    return <span className="text-sky-700">{value}</span>;
  }
  if (typeof value === "boolean") {
    return <span className="text-amber-700">{value ? "true" : "false"}</span>;
  }
  if (typeof value === "undefined") {
    return <span className="text-muted-foreground">undefined</span>;
  }
  return <span className="break-all text-muted-foreground">{String(value)}</span>;
}

function JsonTreeNode({
  label,
  value,
  depth,
  initiallyExpandedDepth,
}: {
  label?: string;
  value: unknown;
  depth: number;
  initiallyExpandedDepth: number;
}) {
  const isArray = Array.isArray(value);
  const isObject = !isArray && value !== null && typeof value === "object";

  if (!isArray && !isObject) {
    return (
      <div className="flex items-start gap-2 font-mono text-xs">
        {label ? <span className="text-muted-foreground">{label}:</span> : null}
        {renderScalar(value)}
      </div>
    );
  }

  const entries = isArray
    ? (value as unknown[]).map((entry, index) => [String(index), entry] as const)
    : Object.entries(value as Record<string, unknown>);

  const summary = isArray ? `[${entries.length}]` : `{${entries.length}}`;

  return (
    <details
      open={depth < initiallyExpandedDepth}
      className="rounded-md border border-dashed border-border/70 bg-muted/30 px-2 py-1"
    >
      <summary className="flex cursor-pointer items-center gap-2 text-xs font-semibold text-foreground/80">
        {label ? <span>{label}</span> : null}
        <span className="text-muted-foreground">{summary}</span>
      </summary>
      <div className="mt-2 space-y-1 border-l border-dashed border-border/60 pl-3">
        {entries.map(([key, entry]) => (
          <JsonTreeNode
            key={`${label ?? "root"}-${key}`}
            label={key}
            value={entry}
            depth={depth + 1}
            initiallyExpandedDepth={initiallyExpandedDepth}
          />
        ))}
      </div>
    </details>
  );
}

export function JsonTreeViewer({
  data,
  rootLabel = "payload",
  initiallyExpandedDepth = 1,
  emptyText = "No JSON payload available.",
}: {
  data: unknown;
  rootLabel?: string;
  initiallyExpandedDepth?: number;
  emptyText?: string;
}) {
  if (data === null || data === undefined) {
    return <p className="text-xs text-muted-foreground">{emptyText}</p>;
  }

  return (
    <div className="space-y-2">
      <JsonTreeNode
        label={rootLabel}
        value={data}
        depth={0}
        initiallyExpandedDepth={initiallyExpandedDepth}
      />
    </div>
  );
}
