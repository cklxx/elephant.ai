"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { StatBlock } from "@/components/eval/stat-block";
import { StatusPill } from "@/components/eval/status-pill";
import { api } from "@/lib/api";
import type { RLStats, QualityTier } from "@/lib/types/rl";

export default function RLDataPage() {
  const [stats, setStats] = useState<RLStats | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .getRLStats()
      .then((data) => setStats(data as RLStats))
      .catch((err: any) => setError(err.message));
  }, []);

  const tiers: QualityTier[] = ["gold", "silver", "bronze", "reject"];

  return (
    <PageShell
      title="RL Data"
      description="Extraction config, quality thresholds, and trajectory catalog."
      actions={
        <a
          href="/rl-data/catalog"
          className="rounded-md border border-border bg-card px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-accent"
        >
          Browse Catalog
        </a>
      }
    >
      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {tiers.map((tier) => {
          const info = stats?.tiers?.[tier];
          return (
            <StatBlock
              key={tier}
              label={tier.charAt(0).toUpperCase() + tier.slice(1)}
              value={info?.total_count ?? 0}
              change={info?.total_bytes ? `${(info.total_bytes / 1024).toFixed(1)} KB` : undefined}
              variant={tier === "gold" ? "success" : tier === "reject" ? "danger" : tier === "bronze" ? "warning" : "default"}
            />
          );
        })}
      </div>

      {stats && (
        <div className="mt-6">
          <h2 className="mb-3 text-sm font-medium text-foreground">Files by Tier</h2>
          {tiers.map((tier) => {
            const info = stats.tiers?.[tier];
            if (!info?.files?.length) return null;
            return (
              <div key={tier} className="mb-4">
                <div className="mb-2 flex items-center gap-2">
                  <StatusPill status={tier} />
                  <span className="text-xs text-muted-foreground">
                    {info.total_count} trajectories
                  </span>
                </div>
                <div className="overflow-x-auto rounded-lg border border-border">
                  <table className="w-full text-left text-sm">
                    <thead className="border-b border-border bg-muted/50">
                      <tr>
                        <th className="px-3 py-2 font-medium text-muted-foreground">File</th>
                        <th className="px-3 py-2 font-medium text-muted-foreground">Count</th>
                        <th className="px-3 py-2 font-medium text-muted-foreground">Size</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border">
                      {info.files.map((f) => (
                        <tr key={f.name}>
                          <td className="px-3 py-2 font-mono text-xs">{f.name}</td>
                          <td className="px-3 py-2 text-xs">{f.count}</td>
                          <td className="px-3 py-2 text-xs">{(f.bytes / 1024).toFixed(1)} KB</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </PageShell>
  );
}
