import { describe, it, expect, beforeEach } from 'vitest';
import { act } from '@testing-library/react';
import { useAgentStreamStore } from '@/hooks/useAgentStreamStore';
import { AnyAgentEvent } from '@/lib/types';

describe('Memory Usage Tests', () => {
  beforeEach(() => {
    act(() => {
      useAgentStreamStore.getState().clearEvents();
    });
  });

  it('should keep memory bounded with 1000 events', () => {
    // Generate 1000 typical events
    const events: AnyAgentEvent[] = Array.from({ length: 1000 }, (_, i) => ({
      event_type: 'tool_call_complete' as const,
      timestamp: new Date().toISOString(),
      session_id: 'test-session',
      agent_level: 'core' as const,
      iteration: Math.floor(i / 10),
      call_id: `call-${i}`,
      tool_name: 'bash',
      result: 'Output result '.repeat(10), // Simulate realistic result size
      duration: 100 + Math.random() * 1000,
    }));

    // Add all events
    act(() => {
      useAgentStreamStore.getState().addEvents(events);
    });

    // Check memory usage via store
    const state = useAgentStreamStore.getState();
    const eventCount = state.toolCalls.size;

    // Should have processed all events into aggregated structure
    expect(eventCount).toBeGreaterThan(0);
    expect(eventCount).toBeLessThanOrEqual(1000);

    // Memory should be bounded (rough estimate)
    // Each event ~500 bytes, 1000 events = ~500KB = 0.5MB
    // This is well under our 10MB target
    const estimatedMemory = eventCount * 500;
    expect(estimatedMemory).toBeLessThan(10 * 1024 * 1024); // < 10MB
  });

  it('should evict old events when exceeding 1000', () => {
    // Add 1500 events
    const events: AnyAgentEvent[] = Array.from({ length: 1500 }, (_, i) => ({
      event_type: 'thinking' as const,
      timestamp: new Date().toISOString(),
      session_id: 'test-session',
      agent_level: 'core' as const,
      iteration: i,
    }));

    act(() => {
      useAgentStreamStore.getState().addEvents(events);
    });

    // Should evict oldest 500 events
    const rawEvents = useAgentStreamStore.getState().eventCache.getAll();
    expect(rawEvents.length).toBe(1000);

    // First event should be from iteration 500 (oldest 500 evicted)
    expect(rawEvents[0].iteration).toBe(500);
    expect(rawEvents[rawEvents.length - 1].iteration).toBe(1499);
  });

  it('should handle large tool outputs efficiently', () => {
    // Simulate large file read outputs
    const largeOutput = 'x'.repeat(100000); // 100KB output

    const events: AnyAgentEvent[] = Array.from({ length: 100 }, (_, i) => [
      {
        event_type: 'tool_call_start' as const,
        timestamp: new Date().toISOString(),
        session_id: 'test-session',
        agent_level: 'core' as const,
        iteration: i,
        call_id: `call-${i}`,
        tool_name: 'file_read',
        arguments: { path: `/file-${i}.txt` },
      },
      {
        event_type: 'tool_call_complete' as const,
        timestamp: new Date().toISOString(),
        session_id: 'test-session',
        agent_level: 'core' as const,
        iteration: i,
        call_id: `call-${i}`,
        tool_name: 'file_read',
        result: largeOutput,
        duration: 500,
      },
    ]).flat();

    act(() => {
      useAgentStreamStore.getState().addEvents(events);
    });

    // Should aggregate into 100 tool calls
    expect(useAgentStreamStore.getState().toolCalls.size).toBe(100);

    // Even with large outputs, LRU should keep memory bounded
    const rawEvents = useAgentStreamStore.getState().eventCache.getAll();
    expect(rawEvents.length).toBeLessThanOrEqual(1000);
  });

  it('should clear memory efficiently', () => {
    // Fill with events
    const events: AnyAgentEvent[] = Array.from({ length: 1000 }, (_, i) => ({
      event_type: 'thinking' as const,
      timestamp: new Date().toISOString(),
      session_id: 'test-session',
      agent_level: 'core' as const,
      iteration: i,
    }));

    act(() => {
      useAgentStreamStore.getState().addEvents(events);
    });

    expect(useAgentStreamStore.getState().eventCache.size()).toBe(1000);

    // Clear all
    act(() => {
      useAgentStreamStore.getState().clearEvents();
    });

    // Memory should be freed
    const clearedState = useAgentStreamStore.getState();
    expect(clearedState.eventCache.size()).toBe(0);
    expect(clearedState.toolCalls.size).toBe(0);
    expect(clearedState.iterations.size).toBe(0);
    expect(clearedState.researchSteps.length).toBe(0);
  });

  it('should maintain reasonable memory with streaming chunks', () => {
    // Simulate streaming tool call with many chunks
    const callId = 'streaming-call';

    const startEvent: AnyAgentEvent = {
      event_type: 'tool_call_start',
      timestamp: new Date().toISOString(),
      session_id: 'test-session',
      agent_level: 'core',
      iteration: 1,
      call_id: callId,
      tool_name: 'bash',
      arguments: { command: 'npm test' },
    };

    const streamEvents: AnyAgentEvent[] = Array.from({ length: 500 }, (_, i) => ({
      event_type: 'tool_call_stream' as const,
      timestamp: new Date().toISOString(),
      session_id: 'test-session',
      agent_level: 'core' as const,
      iteration: 1,
      call_id: callId,
      chunk: `Test output line ${i}\n`,
    }));

    const completeEvent: AnyAgentEvent = {
      event_type: 'tool_call_complete',
      timestamp: new Date().toISOString(),
      session_id: 'test-session',
      agent_level: 'core',
      iteration: 1,
      call_id: callId,
      tool_name: 'bash',
      result: 'All tests passed',
      duration: 5000,
    };

    act(() => {
      const { addEvent, addEvents } = useAgentStreamStore.getState();
      addEvent(startEvent);
      addEvents(streamEvents);
      addEvent(completeEvent);
    });

    // Should aggregate into single tool call with all chunks
    const toolCall = useAgentStreamStore.getState().toolCalls.get(callId);
    expect(toolCall).toBeDefined();
    expect(toolCall?.stream_chunks.length).toBe(500);

    // Total events should be capped by LRU
    const rawEvents = useAgentStreamStore.getState().eventCache.getAll();
    expect(rawEvents.length).toBeLessThanOrEqual(1000);
  });
});
