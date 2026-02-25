"use client";

import { useEffect, useState } from "react";
import { PageShell } from "@/components/layout/page-shell";
import { StatusPill } from "@/components/eval/status-pill";
import { api } from "@/lib/api";
import type { EvaluationJob } from "@/lib/types/evaluation";

export default function EvaluationsPage() {
  const [evaluations, setEvaluations] = useState<EvaluationJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api
      .listEvaluations()
      .then((res) => setEvaluations(res.evaluations ?? []))
      .catch((err: any) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <PageShell
      title="Evaluations"
      description="List and create agent evaluations."
    >
      {error && (
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      )}

      {loading ? (
        <p className="text-sm text-muted-foreground">Loading evaluations...</p>
      ) : evaluations.length === 0 ? (
        <div className="rounded-lg border border-border bg-card p-6 text-center">
          <p className="text-sm text-muted-foreground">
            No evaluations yet. Start one from the eval-server API.
          </p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-border bg-muted/50">
              <tr>
                <th className="px-3 py-2 font-medium text-muted-foreground">ID</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Status</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Agent</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Dataset</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Workers</th>
                <th className="px-3 py-2 font-medium text-muted-foreground">Started</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {evaluations.map((ev) => (
                <tr key={ev.id} className="hover:bg-muted/30">
                  <td className="px-3 py-2">
                    <a
                      href={`/evaluations/${ev.id}`}
                      className="font-mono text-xs text-primary hover:underline"
                    >
                      {ev.id}
                    </a>
                  </td>
                  <td className="px-3 py-2">
                    <StatusPill status={ev.status} />
                  </td>
                  <td className="px-3 py-2 text-xs">{ev.agent_id ?? "–"}</td>
                  <td className="px-3 py-2 text-xs truncate max-w-48">
                    {ev.dataset_path ?? "–"}
                  </td>
                  <td className="px-3 py-2 text-xs">{ev.max_workers ?? "–"}</td>
                  <td className="px-3 py-2 text-xs">
                    {ev.started_at ? new Date(ev.started_at).toLocaleString() : "–"}
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
