import { StatusPill } from "./status-pill";
import type { WorkerResultSummary } from "@/lib/types/evaluation";

interface ResultTableProps {
  results: WorkerResultSummary[];
}

export function ResultTable({ results }: ResultTableProps) {
  if (results.length === 0) {
    return <p className="text-sm text-muted-foreground">No results available.</p>;
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-border">
      <table className="w-full text-left text-sm">
        <thead className="border-b border-border bg-muted/50">
          <tr>
            <th className="px-3 py-2 font-medium text-muted-foreground">Task ID</th>
            <th className="px-3 py-2 font-medium text-muted-foreground">Status</th>
            <th className="px-3 py-2 font-medium text-muted-foreground">Score</th>
            <th className="px-3 py-2 font-medium text-muted-foreground">Grade</th>
            <th className="px-3 py-2 font-medium text-muted-foreground">Duration</th>
            <th className="px-3 py-2 font-medium text-muted-foreground">Tokens</th>
            <th className="px-3 py-2 font-medium text-muted-foreground">Cost</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {results.map((r) => (
            <tr key={r.task_id} className="hover:bg-muted/30">
              <td className="px-3 py-2 font-mono text-xs">{r.task_id}</td>
              <td className="px-3 py-2"><StatusPill status={r.status} /></td>
              <td className="px-3 py-2">{r.auto_score?.toFixed(1) ?? "–"}</td>
              <td className="px-3 py-2">{r.grade ?? "–"}</td>
              <td className="px-3 py-2">{r.duration_seconds ? `${r.duration_seconds.toFixed(1)}s` : "–"}</td>
              <td className="px-3 py-2">{r.tokens_used?.toLocaleString() ?? "–"}</td>
              <td className="px-3 py-2">{r.cost ? `$${r.cost.toFixed(4)}` : "–"}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
