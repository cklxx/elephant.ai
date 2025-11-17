import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { AutoReviewCard } from "../AutoReviewCard";
import { AutoReviewEvent } from "@/lib/types";

describe("AutoReviewCard", () => {
  const baseEvent: AutoReviewEvent = {
    event_type: "auto_review",
    timestamp: new Date().toISOString(),
    agent_level: "core",
    session_id: "s-1",
    task_id: "t-1",
    parent_task_id: undefined,
    summary: {
      assessment: {
        grade: "C",
        score: 0.41,
        needs_rework: true,
        notes: ["answer is extremely short"],
      },
      rework: {
        attempted: 1,
        applied: false,
        notes: ["attempt 1: grade C (0.52)"],
      },
    },
  };

  it("highlights unfinished work when needs_rework is true", () => {
    render(<AutoReviewCard event={baseEvent} />);
    expect(screen.getByText(/仍有未完成内容/)).toBeInTheDocument();
    expect(screen.getByText("未完成内容")).toBeInTheDocument();
    expect(screen.getByText(/Grade C/)).toBeInTheDocument();
    expect(screen.getByText(/answer is extremely short/)).toBeInTheDocument();
  });

  it("renders passing state", () => {
    const passingEvent: AutoReviewEvent = {
      ...baseEvent,
      summary: {
        assessment: {
          grade: "A",
          score: 0.91,
          needs_rework: false,
        },
      },
    };
    render(<AutoReviewCard event={passingEvent} />);
    expect(screen.getByText(/任务已通过/)).toBeInTheDocument();
    expect(screen.getByText(/Score 0.91/)).toBeInTheDocument();
  });

  it("emits follow-up intents when action buttons are clicked", () => {
    const onAction = vi.fn();
    render(<AutoReviewCard event={baseEvent} onAction={onAction} />);

    const continueButton = screen.getByRole("button", { name: "继续完成" });
    fireEvent.click(continueButton);
    expect(onAction).toHaveBeenCalledWith({ action: "continue", event: baseEvent });

    const continueWithNotesButton = screen.getByRole("button", {
      name: "继续完成并标注当前项目未完成内容",
    });
    fireEvent.click(continueWithNotesButton);
    expect(onAction).toHaveBeenCalledWith({
      action: "continue_with_notes",
      event: baseEvent,
    });
  });
});
