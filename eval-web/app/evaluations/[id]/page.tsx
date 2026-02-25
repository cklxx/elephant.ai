"use client";

import { useEffect, useState } from "react";
import { useParams } from "next/navigation";
import { PageShell } from "@/components/layout/page-shell";
import { StatBlock } from "@/components/eval/stat-block";
import { StatusPill } from "@/components/eval/status-pill";
import { ResultTable } from "@/components/eval/result-table";
import { api } from "@/lib/api";
import type { EvaluationDetail } from "@/lib/types/evaluation";

export default function EvaluationDetailPage() {
  const params = useParams();
  const id = params?.id as string;
  const [detail, setDetail] = useState<EvaluationDetail | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    api
      .getEvaluation(id)
      .then((res) => setDetail(res as EvaluationDetail))
      .catch((err: any) => setError(err.message));
  }, [id]);

  if (error) {
    return (
      <PageShell title="Evaluation Detail">
        <div className="rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-sm text-destructive">{error}</p>
        </div>
      </PageShell>
    );
  }

  if (!detail) {
    return (
      <PageShell title="Evaluation Detail">
        <p className="text-sm text-muted-foreground">Loading...</p>
      </PageShell>
    );
  }

  const ev = detail.evaluation;
  const summary = ev.summary;

  return (
    <PageShell
      title={`Evaluation ${ev.id}`}
      description={ev.agent_id ? `Agent: ${ev.agent_id}` : undefined}
    >
      <div className="flex items-center gap-3">
        <StatusPill status={ev.status} />
        {ev.started_at && (
          <span className="text-xs text-muted-foreground">
            Started {new Date(ev.started_at).toLocaleString()}
          </span>
        )}
      </div>

      {summary && (
        <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatBlock label="Overall Score" value={summary.overall_score.toFixed(1)} />
          <StatBlock
            label="Success Rate"
            value={`${(summary.success_rate * 100).toFixed(0)}%`}
            variant={summary.success_rate >= 0.7 ? "success" : summary.success_rate >= 0.4 ? "warning" : "danger"}
          />
          <StatBlock label="Completed" value={summary.completed_tasks} variant="success" />
          <StatBlock label="Failed" value={summary.failed_tasks} variant="danger" />
        </div>
      )}

      {detail.results && detail.results.length > 0 && (
        <div className="mt-6">
          <h2 className="mb-3 text-sm font-medium text-foreground">Results</h2>
          <ResultTable results={detail.results} />
        </div>
      )}

      {ev.error && (
        <div className="mt-4 rounded-lg border border-destructive/50 bg-destructive/10 p-3">
          <p className="text-xs font-medium text-destructive">Error: {ev.error}</p>
        </div>
      )}
    </PageShell>
  );
}
