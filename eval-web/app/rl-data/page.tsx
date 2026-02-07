import { PageShell } from "@/components/layout/page-shell";

export default function RLDataPage() {
  return (
    <PageShell
      title="RL Data"
      description="Extraction config, quality thresholds, and trajectory catalog."
    >
      <div className="rounded-lg border border-border bg-card p-6">
        <p className="text-sm text-muted-foreground">
          RL data pipeline UI coming in Batch 6.
        </p>
      </div>
    </PageShell>
  );
}
