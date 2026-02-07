"use client";

import { PageShell } from "@/components/layout/page-shell";

export default function SessionsPage() {
  return (
    <PageShell
      title="Sessions"
      description="Browse and inspect agent sessions with trace replay."
    >
      <div className="rounded-lg border border-border bg-card p-6">
        <p className="text-sm text-muted-foreground">
          Session browser requires connecting to session store. Configure
          <code className="mx-1 rounded bg-muted px-1 py-0.5 font-mono text-xs">
            session_dir
          </code>
          in eval-server config to enable.
        </p>
      </div>

      <div className="mt-4 rounded-lg border border-border bg-card p-4">
        <h3 className="text-sm font-medium text-foreground">Quick Actions</h3>
        <div className="mt-3 flex flex-wrap gap-2">
          <ActionButton label="Browse Sessions" disabled />
          <ActionButton label="Replay Session" disabled />
          <ActionButton label="Add Annotation" disabled />
        </div>
      </div>
    </PageShell>
  );
}

function ActionButton({ label, disabled }: { label: string; disabled?: boolean }) {
  return (
    <button
      disabled={disabled}
      className="rounded-md border border-border bg-card px-3 py-1.5 text-xs font-medium text-muted-foreground transition-colors hover:bg-accent disabled:opacity-50"
    >
      {label}
    </button>
  );
}
