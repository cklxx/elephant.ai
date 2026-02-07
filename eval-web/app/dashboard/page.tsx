import { PageShell } from "@/components/layout/page-shell";

export default function DashboardPage() {
  return (
    <PageShell
      title="Dashboard"
      description="Overview of evaluation stats, agent trends, and RL data counts."
    >
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard label="Total Evaluations" value="--" />
        <StatCard label="Active Agents" value="--" />
        <StatCard label="RL Trajectories" value="--" />
        <StatCard label="Sessions" value="--" />
      </div>
      <div className="mt-6 rounded-lg border border-border bg-card p-6">
        <p className="text-sm text-muted-foreground">
          Connect to eval-server to see live data. Detailed charts and trends coming in Batch 5.
        </p>
      </div>
    </PageShell>
  );
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-border bg-card p-4">
      <p className="text-xs font-medium text-muted-foreground">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-foreground">{value}</p>
    </div>
  );
}
