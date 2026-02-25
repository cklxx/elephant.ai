"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";

const API_URL = process.env.NEXT_PUBLIC_EVAL_API_URL ?? "http://localhost:8081";

export default function ConfigPage() {
  const [config, setConfig] = useState<Record<string, any> | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [editing, setEditing] = useState(false);
  const [editText, setEditText] = useState("");

  useEffect(() => {
    fetch(`${API_URL}/api/internal/config/runtime`)
      .then((r) => r.json())
      .then(setConfig)
      .catch((err) => setError(err.message));
  }, []);

  function startEdit() {
    setEditText(JSON.stringify(config, null, 2));
    setEditing(true);
  }

  async function saveConfig() {
    try {
      const parsed = JSON.parse(editText);
      const res = await fetch(`${API_URL}/api/internal/config/runtime`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(parsed),
      });
      if (!res.ok) throw new Error(await res.text());
      setConfig(parsed);
      setEditing(false);
    } catch (err: any) {
      setError(err.message);
    }
  }

  return (
    <PageShell
      title="Runtime Config"
      description="View and edit runtime configuration."
      actions={
        editing ? (
          <div className="flex gap-2">
            <button onClick={() => setEditing(false)} className="rounded-md border border-border px-3 py-1.5 text-xs">Cancel</button>
            <button onClick={saveConfig} className="rounded-md border border-primary bg-primary px-3 py-1.5 text-xs text-primary-foreground">Save</button>
          </div>
        ) : (
          <button onClick={startEdit} className="rounded-md border border-border px-3 py-1.5 text-xs hover:bg-accent">Edit</button>
        )
      }
    >
      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      {editing ? (
        <textarea
          value={editText}
          onChange={(e) => setEditText(e.target.value)}
          className="h-[500px] w-full rounded-lg border border-border bg-card p-4 font-mono text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
        />
      ) : config ? (
        <pre className="overflow-auto rounded-lg border border-border bg-card p-4 font-mono text-xs text-foreground">
          {JSON.stringify(config, null, 2)}
        </pre>
      ) : (
        <p className="text-sm text-muted-foreground">Loading config...</p>
      )}
    </PageShell>
  );
}
