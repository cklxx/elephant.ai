"use client";

import { AutoReviewActionIntent, AutoReviewEvent } from "@/lib/types";
import { cn } from "@/lib/utils";
import { collectOutstandingReviewNotes } from "@/lib/autoReview";

interface AutoReviewCardProps {
  event: AutoReviewEvent;
  onAction?: (intent: AutoReviewActionIntent) => void;
}

export function AutoReviewCard({ event, onAction }: AutoReviewCardProps) {
  const summary = event.summary;
  const assessment = summary?.assessment;
  const rework = summary?.rework;
  const needsWork = assessment?.needs_rework ?? false;
  const showActions = needsWork && typeof onAction === "function";
  const badgeClass = needsWork
    ? "border-amber-200 bg-amber-100 text-amber-900"
    : "border-emerald-200 bg-emerald-100 text-emerald-900";
  const headline = needsWork
    ? "自动评审：仍有未完成内容，需要继续完成"
    : "自动评审：任务已通过";
  const helper = needsWork
    ? "Auto-review flagged outstanding gaps — consider running another task to continue the project."
    : "Auto-review confirmed the final answer meets the expected quality bar.";
  const outstandingNotes = collectOutstandingReviewNotes(summary);
  const hasOutstandingNotes = needsWork && outstandingNotes.length > 0;

  const triggerAction = (action: AutoReviewActionIntent["action"]) => {
    onAction?.({ action, event });
  };

  return (
    <div
      className={cn(
        "mt-3 rounded-lg border px-3 py-3 text-sm",
        needsWork ? "border-amber-200 bg-amber-50" : "border-emerald-200 bg-emerald-50",
      )}
      data-testid="event-auto_review"
    >
      <div className="flex items-center justify-between gap-3">
        <p className="font-semibold text-slate-700">{headline}</p>
        <span className={cn("rounded-full border px-2 py-0.5 text-xs font-semibold", badgeClass)}>
          {assessment?.grade ? `Grade ${assessment.grade}` : "Grade N/A"}
        </span>
      </div>
      <p className="mt-1 text-xs text-slate-500">{helper}</p>

      {assessment?.score !== undefined && (
        <p className="mt-2 text-xs uppercase tracking-wide text-slate-400">
          Score {assessment.score.toFixed(2)}
        </p>
      )}

      {hasOutstandingNotes && (
        <div className="mt-3 rounded-md border border-amber-200/70 bg-amber-50/70 px-3 py-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-amber-900">
            未完成内容
          </p>
          <ul className="mt-1 list-disc space-y-1 pl-4 text-sm text-amber-900">
            {outstandingNotes.map((note, index) => (
              <li key={`review-outstanding-note-${index}`}>{note}</li>
            ))}
          </ul>
        </div>
      )}

      {rework && (
        <div className="mt-3 rounded-md border border-white/60 bg-white/60 px-3 py-2">
          <p className="text-xs font-semibold uppercase tracking-wide text-slate-500">
            Rework summary
          </p>
          <p className="mt-1 text-sm text-slate-600">
            Attempts: {rework.attempted ?? 0}
            {rework.final_grade && (
              <>
                {" "}| Final {rework.final_grade}
                {rework.final_score !== undefined && ` (${rework.final_score.toFixed(2)})`}
              </>
            )}
            {rework.applied && " | latest answer applied"}
          </p>
          {rework.notes && rework.notes.length > 0 && (
            <ul className="mt-1 list-disc space-y-1 pl-4 text-xs text-slate-500">
              {rework.notes.map((note, index) => (
                <li key={`rework-note-${index}`}>{note}</li>
              ))}
            </ul>
          )}
        </div>
      )}

      {showActions && (
        <div className="mt-3 flex flex-wrap gap-2">
          <button
            type="button"
            className="inline-flex items-center justify-center rounded-md border border-amber-200 bg-white px-3 py-1.5 text-xs font-semibold text-amber-900 shadow-sm transition hover:bg-amber-50 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-amber-400"
            onClick={() => triggerAction("continue")}
          >
            继续完成
          </button>
          <button
            type="button"
            className="inline-flex items-center justify-center rounded-md border border-slate-900 bg-slate-900 px-3 py-1.5 text-xs font-semibold text-white shadow-sm transition hover:bg-slate-800 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-slate-500"
            onClick={() => triggerAction("continue_with_notes")}
          >
            继续完成并标注当前项目未完成内容
          </button>
        </div>
      )}
    </div>
  );
}
