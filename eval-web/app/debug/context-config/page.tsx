"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";

const API_URL = process.env.NEXT_PUBLIC_EVAL_API_URL ?? "http://localhost:8081";

export default function ContextConfigPage() {
  const [config, setConfig] = useState<Record<string, any> | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetch(`${API_URL}/api/dev/context-config`)
      .then((r) => r.json())
      .then(setConfig)
      .catch((err) => setError(err.message));
  }, []);

  return (
    <PageShell
      title="Context Config"
      description="View context window section configuration."
    >
      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      {config ? (
        <pre className="overflow-auto rounded-lg border border-border bg-card p-4 font-mono text-xs text-foreground">
          {JSON.stringify(config, null, 2)}
        </pre>
      ) : !error ? (
        <p className="text-sm text-muted-foreground">Loading context config...</p>
      ) : null}
    </PageShell>
  );
}
