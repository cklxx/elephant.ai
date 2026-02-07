"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { StatBlock } from "@/components/eval/stat-block";
import { api } from "@/lib/api";

interface DashboardData {
  evaluationCount: number;
  agentCount: number;
  rlStats: { gold: number; silver: number; bronze: number; reject: number };
  health: string;
}

export default function DashboardPage() {
  const [data, setData] = useState<DashboardData | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function load() {
      try {
        const [healthRes, evalRes, agentRes, rlRes] = await Promise.allSettled([
          api.health(),
          api.listEvaluations(),
          api.listAgents(),
          api.getRLStats(),
        ]);

        const health = healthRes.status === "fulfilled" ? healthRes.value.status : "unreachable";
        const evals = evalRes.status === "fulfilled" ? evalRes.value.evaluations : [];
        const agents = agentRes.status === "fulfilled" ? agentRes.value.agents : [];

        let rlCounts = { gold: 0, silver: 0, bronze: 0, reject: 0 };
        if (rlRes.status === "fulfilled" && rlRes.value?.tiers) {
          const tiers = rlRes.value.tiers;
          rlCounts = {
            gold: tiers.gold?.total_count ?? 0,
            silver: tiers.silver?.total_count ?? 0,
            bronze: tiers.bronze?.total_count ?? 0,
            reject: tiers.reject?.total_count ?? 0,
          };
        }

        setData({
          evaluationCount: evals.length,
          agentCount: agents.length,
          rlStats: rlCounts,
          health,
        });
      } catch (err: any) {
        setError(err.message ?? "Failed to load dashboard data");
      }
    }
    load();
  }, []);

  const totalRL = data
    ? data.rlStats.gold + data.rlStats.silver + data.rlStats.bronze
    : 0;

  return (
    <PageShell
      title="Dashboard"
      description="Overview of evaluation stats, agent trends, and RL data counts."
    >
      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatBlock label="Evaluations" value={data?.evaluationCount ?? "–"} />
        <StatBlock label="Agents" value={data?.agentCount ?? "–"} />
        <StatBlock label="RL Trajectories" value={totalRL || "–"} />
        <StatBlock
          label="Server Status"
          value={data?.health ?? "–"}
          variant={data?.health === "ok" ? "success" : "warning"}
        />
      </div>

      {data && (
        <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatBlock label="Gold" value={data.rlStats.gold} variant="success" />
          <StatBlock label="Silver" value={data.rlStats.silver} />
          <StatBlock label="Bronze" value={data.rlStats.bronze} variant="warning" />
          <StatBlock label="Rejected" value={data.rlStats.reject} variant="danger" />
        </div>
      )}
    </PageShell>
  );
}
