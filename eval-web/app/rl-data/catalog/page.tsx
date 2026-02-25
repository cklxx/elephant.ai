"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { StatusPill } from "@/components/eval/status-pill";
import { api } from "@/lib/api";
import type { RLTrajectory, QualityTier } from "@/lib/types/rl";

export default function RLCatalogPage() {
  const [tier, setTier] = useState<QualityTier>("gold");
  const [trajectories, setTrajectories] = useState<RLTrajectory[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    api
      .listTrajectories({ tier })
      .then((data) => setTrajectories(data.trajectories ?? []))
      .catch((err: any) => setError(err.message))
      .finally(() => setLoading(false));
  }, [tier]);

  const tiers: QualityTier[] = ["gold", "silver", "bronze", "reject"];

  return (
    <PageShell
      title="RL Catalog"
      description="Browse and export JSONL trajectories by quality tier."
      actions={
        <a
          href={`http://localhost:8081/api/rl/export?tier=${tier}`}
          target="_blank"
          rel="noopener noreferrer"
          className="rounded-md border border-border bg-card px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-accent"
        >
          Export {tier} JSONL
        </a>
      }
    >
      <div className="flex gap-2">
        {tiers.map((t) => (
          <button
            key={t}
            onClick={() => setTier(t)}
            className={`rounded-md border px-3 py-1.5 text-xs font-medium transition-colors ${
              tier === t
                ? "border-primary bg-primary text-primary-foreground"
                : "border-border bg-card text-muted-foreground hover:bg-accent"
            }`}
          >
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </button>
        ))}
      </div>

      {error && (
        <div className="mt-4 rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      {loading ? (
        <p className="mt-4 text-sm text-muted-foreground">Loading trajectories...</p>
      ) : trajectories.length === 0 ? (
        <div className="mt-4 rounded-lg border border-border bg-card p-6 text-center">
          <p className="text-sm text-muted-foreground">
            No {tier} trajectories found.
          </p>
        </div>
      ) : (
        <div className="mt-4 overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/50">
              <tr>
                <th className="px-3 py-2 font-medium text-muted-foreground">ID</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Task</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Tier</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Auto Score</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Judge</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Steps</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Outcome</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Extracted</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {trajectories.map((traj) => (
                <tr key={traj.id} className="hover:bg-muted/30">
                  <td className="px-3 py-2 font-mono text-xs">{traj.id}</td>
                  <td className="px-3 py-2 text-xs">{traj.task_id}</td>
                  <td className="px-3 py-2"><StatusPill status={traj.quality_tier} /></td>
                  <td className="px-3 py-2 text-xs">{traj.auto_score.toFixed(1)}</td>
                  <td className="px-3 py-2 text-xs">
                    {traj.judge_score != null ? (
                      <span className="text-primary">{traj.judge_score.toFixed(1)}</span>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </td>
                  <td className="px-3 py-2 text-xs">{traj.metadata.total_steps}</td>
                  <td className="px-3 py-2"><StatusPill status={traj.metadata.outcome} /></td>
                  <td className="px-3 py-2 text-xs">
                    {new Date(traj.extracted_at).toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </PageShell>
  );
}
