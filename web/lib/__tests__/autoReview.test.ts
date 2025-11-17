import { describe, expect, it } from "vitest";
import { collectOutstandingReviewNotes } from "@/lib/autoReview";

describe("collectOutstandingReviewNotes", () => {
  it("deduplicates and trims notes across assessment and rework", () => {
    const notes = collectOutstandingReviewNotes({
      assessment: {
        grade: "C",
        score: 0.42,
        needs_rework: true,
        notes: [" fix dashboard layout ", "Add docs"],
      },
      rework: {
        attempted: 1,
        applied: false,
        notes: ["Add docs", "ship QA steps"],
      },
    });

    expect(notes).toEqual([
      "fix dashboard layout",
      "Add docs",
      "ship QA steps",
    ]);
  });

  it("returns empty array when summary missing", () => {
    expect(collectOutstandingReviewNotes(undefined)).toEqual([]);
  });
});

