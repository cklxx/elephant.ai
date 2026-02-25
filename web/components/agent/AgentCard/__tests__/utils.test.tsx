import { describe, it, expect } from "vitest";
import { subagentThreadToCardData } from "../utils";
import { SubagentContext } from "../../EventLine";
import { AnyAgentEvent } from "@/lib/types";

describe("subagentThreadToCardData", () => {
  it("converts basic subagent thread to card data", () => {
    const context: SubagentContext = {
      preview: "Test Task",
      stats: "5 tool calls · 1000 tokens",
    };

    const events: AnyAgentEvent[] = [
      {
        event_type: "workflow.tool.completed",
        timestamp: "2024-01-01T00:00:00Z",
        agent_level: "subagent",
        session_id: "session-test",
        call_id: "call-1",
        tool_name: "test",
        result: "",
        duration: 0,
      },
    ];

    const result = subagentThreadToCardData("test-key", context, events, 0);

    expect(result.id).toBe("test-key");
    expect(result.preview).toBe("Test Task");
    expect(result.stats.toolCalls).toBe(5);
    expect(result.stats.tokens).toBe(1000);
    expect(result.events).toEqual(events);
  });

  it("parses progress information", () => {
    const context: SubagentContext = {
      progress: "Progress 7/10",
    };

    const result = subagentThreadToCardData("key", context, [], 0);

    expect(result.progress).toEqual({
      current: 7,
      total: 10,
      percentage: 70,
    });
  });

  it("parses concurrency information", () => {
    const context: SubagentContext = {
      concurrency: "Parallel ×3",
    };

    const result = subagentThreadToCardData("key", context, [], 2);

    expect(result.concurrency).toEqual({
      index: 3,
      total: 3,
    });
  });

  it("infers running state from events", () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: "workflow.tool.started",
        timestamp: "2024-01-01T00:00:00Z",
        agent_level: "subagent",
        session_id: "session-test",
        call_id: "call-1",
        tool_name: "test",
        arguments: {},
      },
    ];

    const result = subagentThreadToCardData("key", {}, events, 0);

    expect(result.state).toBe("running");
  });

  it("infers completed state from final result event", () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: "workflow.result.final",
        timestamp: "2024-01-01T00:00:00Z",
        agent_level: "subagent",
        session_id: "session-test",
        final_answer: "",
        total_iterations: 0,
        total_tokens: 0,
        stop_reason: "done",
        duration: 0,
      },
    ];

    const result = subagentThreadToCardData("key", {}, events, 0);

    expect(result.state).toBe("completed");
  });

  it("infers failed state from failed event", () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: "workflow.node.failed",
        timestamp: "2024-01-01T00:00:00Z",
        agent_level: "subagent",
        session_id: "session-test",
        error: "test error",
      },
    ];

    const result = subagentThreadToCardData("key", {}, events, 0);

    expect(result.state).toBe("failed");
  });

  it("infers cancelled state from cancelled event", () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: "workflow.result.cancelled",
        timestamp: "2024-01-01T00:00:00Z",
        agent_level: "subagent",
        session_id: "session-test",
        reason: "user",
      },
    ];

    const result = subagentThreadToCardData("key", {}, events, 0);

    expect(result.state).toBe("cancelled");
  });

  it("returns idle state for empty events", () => {
    const result = subagentThreadToCardData("key", {}, [], 0);

    expect(result.state).toBe("idle");
  });

  it("handles missing optional fields gracefully", () => {
    const result = subagentThreadToCardData("key", {}, [], 0);

    expect(result.preview).toBeUndefined();
    expect(result.progress).toBeUndefined();
    expect(result.concurrency).toBeUndefined();
    expect(result.stats.toolCalls).toBe(0);
    expect(result.stats.tokens).toBe(0);
  });

  it("does not create concurrency info for single agent", () => {
    const context: SubagentContext = {
      concurrency: "Parallel ×1",
    };

    const result = subagentThreadToCardData("key", context, [], 0);

    expect(result.concurrency).toBeUndefined();
  });
});
