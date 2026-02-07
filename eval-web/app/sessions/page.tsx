import { PageShell } from "@/components/layout/page-shell";

export default function SessionsPage() {
  return (
    <PageShell
      title="Sessions"
      description="Browse and inspect agent sessions."
    >
      <div className="rounded-lg border border-border bg-card p-6">
        <p className="text-sm text-muted-foreground">
          Session browser coming in Batch 6.
        </p>
      </div>
    </PageShell>
  );
}
