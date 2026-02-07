import { PageShell } from "@/components/layout/page-shell";

export default function EvaluationsPage() {
  return (
    <PageShell
      title="Evaluations"
      description="List and create agent evaluations."
    >
      <div className="rounded-lg border border-border bg-card p-6">
        <p className="text-sm text-muted-foreground">
          Evaluation list and create form coming in Batch 5.
        </p>
      </div>
    </PageShell>
  );
}
