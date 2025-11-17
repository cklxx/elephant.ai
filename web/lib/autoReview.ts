import { AutoReviewSummary } from "@/lib/types";

/**
 * collectOutstandingReviewNotes extracts and normalizes reviewer notes that
 * describe unfinished work so downstream surfaces can prefill follow-up tasks
 * or render clear callouts for reviewers.
 */
export function collectOutstandingReviewNotes(
  summary?: AutoReviewSummary | null,
): string[] {
  if (!summary) {
    return [];
  }

  const deduped = new Set<string>();
  const notes: string[] = [];

  const push = (value?: string | null) => {
    if (typeof value !== "string") {
      return;
    }
    const trimmed = value.trim();
    if (!trimmed || deduped.has(trimmed)) {
      return;
    }
    deduped.add(trimmed);
    notes.push(trimmed);
  };

  summary.assessment?.notes?.forEach(push);
  summary.rework?.notes?.forEach(push);

  return notes;
}

