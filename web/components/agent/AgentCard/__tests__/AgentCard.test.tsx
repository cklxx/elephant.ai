import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { AgentCard } from "../index";
import { AgentCardData } from "../types";

describe("AgentCard", () => {
  const mockCardData: AgentCardData = {
    id: "test-agent-1",
    state: "running",
    preview: "Test Agent Preview",
    type: "Explore",
    progress: {
      current: 5,
      total: 10,
      percentage: 50,
    },
    stats: {
      toolCalls: 12,
      tokens: 3200,
    },
    events: [
      {
        event_type: "workflow.tool.completed",
        timestamp: "2024-01-01T00:00:00Z",
        agent_level: "subagent",
      },
      {
        event_type: "workflow.tool.completed",
        timestamp: "2024-01-01T00:00:01Z",
        agent_level: "subagent",
      },
    ],
  };

  it("renders agent card with basic information", () => {
    render(<AgentCard data={mockCardData} />);

    expect(screen.getByTestId("subagent-thread")).toBeInTheDocument();
    expect(screen.getByText("Test Agent Preview")).toBeInTheDocument();
    expect(screen.getByText("Running")).toBeInTheDocument();
  });

  it("displays progress information", () => {
    render(<AgentCard data={mockCardData} />);

    expect(screen.getByText(/Progress: 5\/10/)).toBeInTheDocument();
    expect(screen.getByText("50%")).toBeInTheDocument();
  });

  it("displays stats information", () => {
    render(<AgentCard data={mockCardData} />);

    expect(screen.getByText(/12 tool calls/)).toBeInTheDocument();
    expect(screen.getByText(/3.2K tokens/)).toBeInTheDocument();
  });

  it("toggles event display on footer button click", () => {
    render(<AgentCard data={mockCardData} />);

    const toggleButton = screen.getByRole("button", { name: /Show events/ });
    expect(toggleButton).toBeInTheDocument();

    fireEvent.click(toggleButton);

    expect(screen.getByRole("button", { name: /Hide events/ })).toBeInTheDocument();
  });

  it("renders with controlled expanded state", () => {
    const onToggle = vi.fn();
    const { rerender } = render(
      <AgentCard data={mockCardData} expanded={false} onToggleExpand={onToggle} />,
    );

    expect(screen.getByRole("button", { name: /Show events/ })).toBeInTheDocument();

    rerender(
      <AgentCard data={mockCardData} expanded={true} onToggleExpand={onToggle} />,
    );

    expect(screen.getByRole("button", { name: /Hide events/ })).toBeInTheDocument();
  });

  it("displays concurrency badge when applicable", () => {
    const dataWithConcurrency: AgentCardData = {
      ...mockCardData,
      concurrency: {
        index: 2,
        total: 3,
      },
    };

    render(<AgentCard data={dataWithConcurrency} />);

    expect(screen.getByText("2/3")).toBeInTheDocument();
    expect(screen.getByText(/Parallel ×3/)).toBeInTheDocument();
  });

  it("renders completed state correctly", () => {
    const completedData: AgentCardData = {
      ...mockCardData,
      state: "completed",
    };

    render(<AgentCard data={completedData} />);

    expect(screen.getByText("Completed")).toBeInTheDocument();
    const card = screen.getByTestId("subagent-thread");
    expect(card).toHaveAttribute("data-agent-state", "completed");
  });

  it("renders failed state correctly", () => {
    const failedData: AgentCardData = {
      ...mockCardData,
      state: "failed",
    };

    render(<AgentCard data={failedData} />);

    expect(screen.getByText("Failed")).toBeInTheDocument();
  });

  it("does not show footer when no events", () => {
    const noEventsData: AgentCardData = {
      ...mockCardData,
      events: [],
    };

    render(<AgentCard data={noEventsData} />);

    expect(screen.queryByRole("button", { name: /Show events/ })).not.toBeInTheDocument();
  });

  it("displays state icon", () => {
    render(<AgentCard data={mockCardData} />);

    const stateIcon = screen.getByRole("img", { name: "running" });
    expect(stateIcon).toBeInTheDocument();
  });
});
