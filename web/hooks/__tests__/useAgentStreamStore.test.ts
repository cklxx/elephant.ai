import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useAgentStreamStore, useMemoryStats } from '../useAgentStreamStore';
import { AnyAgentEvent } from '@/lib/types';

describe('useAgentStreamStore', () => {
  beforeEach(() => {
    // Reset store before each test
    const { result } = renderHook(() => useAgentStreamStore());
    act(() => {
      result.current.clearEvents();
    });
  });

  describe('LRU Event Caching', () => {
    it('should add events to cache', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const event: AnyAgentEvent = {
        event_type: 'task_analysis',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        action_name: 'Test Action',
        goal: 'Test Goal',
      };

      act(() => {
        result.current.addEvent(event);
      });

      const { result: memoryResult } = renderHook(() => useMemoryStats());
      expect(memoryResult.current.eventCount).toBe(1);
    });

    it('should evict oldest events when cache exceeds 1000 events', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      // Add 1001 events
      const events: AnyAgentEvent[] = Array.from({ length: 1001 }, (_, i) => ({
        event_type: 'thinking',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: i,
      }));

      act(() => {
        result.current.addEvents(events);
      });

      const { result: memoryResult } = renderHook(() => useMemoryStats());

      // Should have exactly 1000 events (LRU eviction)
      expect(memoryResult.current.eventCount).toBe(1000);
    });

    it('should maintain memory bounds under 5MB for typical events', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      // Add 1000 typical events
      const events: AnyAgentEvent[] = Array.from({ length: 1000 }, (_, i) => ({
        event_type: 'tool_call_start',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: i,
        call_id: `call-${i}`,
        tool_name: 'file_read',
        arguments: { path: '/test/file.txt' },
      }));

      act(() => {
        result.current.addEvents(events);
      });

      const { result: memoryResult } = renderHook(() => useMemoryStats());

      // Estimated memory should be under 5MB (5 * 1024 * 1024 bytes)
      expect(memoryResult.current.estimatedBytes).toBeLessThan(5 * 1024 * 1024);
    });

    it('should handle event deduplication by call_id', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const startEvent: AnyAgentEvent = {
        event_type: 'tool_call_start',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-123',
        tool_name: 'bash',
        arguments: { command: 'ls' },
      };

      const streamEvent: AnyAgentEvent = {
        event_type: 'tool_call_stream',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-123',
        chunk: 'file1.txt\n',
      };

      const completeEvent: AnyAgentEvent = {
        event_type: 'tool_call_complete',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-123',
        result: 'file1.txt\nfile2.txt',
        duration: 100,
      };

      act(() => {
        result.current.addEvent(startEvent);
        result.current.addEvent(streamEvent);
        result.current.addEvent(completeEvent);
      });

      // Should aggregate into single tool call
      expect(result.current.toolCalls.size).toBe(1);
      const toolCall = result.current.toolCalls.get('call-123');
      expect(toolCall?.status).toBe('complete');
      expect(toolCall?.stream_chunks).toHaveLength(1);
    });
  });

  describe('Step Tree Aggregation', () => {
    it('should track research steps correctly', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const planEvent: AnyAgentEvent = {
        event_type: 'research_plan',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        plan_steps: ['Step 1: Research', 'Step 2: Implement', 'Step 3: Test'],
        estimated_iterations: 5,
      };

      const stepStartedEvent: AnyAgentEvent = {
        event_type: 'step_started',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        step_index: 0,
        step_description: 'Step 1: Research',
      };

      const stepCompletedEvent: AnyAgentEvent = {
        event_type: 'step_completed',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 2,
        step_index: 0,
        step_result: 'Research completed successfully',
      };

      act(() => {
        result.current.addEvent(planEvent);
        result.current.addEvent(stepStartedEvent);
        result.current.addEvent(stepCompletedEvent);
      });

      expect(result.current.researchSteps).toHaveLength(1);
      expect(result.current.researchSteps[0].status).toBe('completed');
      expect(result.current.researchSteps[0].result).toBe('Research completed successfully');
    });

    it('should group events by iteration', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const iterStartEvent: AnyAgentEvent = {
        event_type: 'iteration_start',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iterations: 5,
      };

      const thinkingEvent: AnyAgentEvent = {
        event_type: 'thinking',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
      };

      const iterCompleteEvent: AnyAgentEvent = {
        event_type: 'iteration_complete',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        tokens_used: 500,
        tools_run: 2,
      };

      act(() => {
        result.current.addEvent(iterStartEvent);
        result.current.addEvent(thinkingEvent);
        result.current.addEvent(iterCompleteEvent);
      });

      expect(result.current.iterations.size).toBe(1);
      const iteration = result.current.iterations.get(1);
      expect(iteration?.status).toBe('complete');
      expect(iteration?.tokens_used).toBe(500);
    });
  });

  describe('Task Status Management', () => {
    it('should transition from idle to analyzing', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      expect(result.current.taskStatus).toBe('idle');

      const analysisEvent: AnyAgentEvent = {
        event_type: 'task_analysis',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        action_name: 'Test Task',
        goal: 'Complete the test',
      };

      act(() => {
        result.current.addEvent(analysisEvent);
      });

      expect(result.current.taskStatus).toBe('analyzing');
      expect(result.current.taskAnalysis.action_name).toBe('Test Task');
    });

    it('should transition to running on iteration start', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const iterStartEvent: AnyAgentEvent = {
        event_type: 'iteration_start',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iterations: 3,
      };

      act(() => {
        result.current.addEvent(iterStartEvent);
      });

      expect(result.current.taskStatus).toBe('running');
      expect(result.current.currentIteration).toBe(1);
    });

    it('should transition to completed on task complete', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const completeEvent: AnyAgentEvent = {
        event_type: 'task_complete',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 3,
        final_answer: 'Task completed successfully',
        stop_reason: 'completed',
        total_iterations: 3,
        total_tokens: 1500,
        duration: 30000,
      };

      act(() => {
        result.current.addEvent(completeEvent);
      });

      expect(result.current.taskStatus).toBe('completed');
      expect(result.current.finalAnswer).toBe('Task completed successfully');
      expect(result.current.totalIterations).toBe(3);
      expect(result.current.totalTokens).toBe(1500);
    });

    it('should transition to error on error event', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const errorEvent: AnyAgentEvent = {
        event_type: 'error',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        phase: 'execute',
        error: 'Something went wrong',
        recoverable: false,
      };

      act(() => {
        result.current.addEvent(errorEvent);
      });

      expect(result.current.taskStatus).toBe('error');
      expect(result.current.errorMessage).toBe('Something went wrong');
    });
  });

  describe('Active Tool Call Tracking', () => {
    it('should track active tool call', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const startEvent: AnyAgentEvent = {
        event_type: 'tool_call_start',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-456',
        tool_name: 'grep',
        arguments: { pattern: 'test', path: '.' },
      };

      act(() => {
        result.current.addEvent(startEvent);
      });

      expect(result.current.activeToolCall).toBe('call-456');

      const completeEvent: AnyAgentEvent = {
        event_type: 'tool_call_complete',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-456',
        result: 'Found 5 matches',
        duration: 250,
      };

      act(() => {
        result.current.addEvent(completeEvent);
      });

      expect(result.current.activeToolCall).toBe(null);
    });
  });

  describe('Clear Events', () => {
    it('should reset all state when clearing events', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      // Add some events
      const events: AnyAgentEvent[] = [
        {
          event_type: 'task_analysis',
          timestamp: new Date().toISOString(),
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          action_name: 'Test',
          goal: 'Test goal',
        },
        {
          event_type: 'iteration_start',
          timestamp: new Date().toISOString(),
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iterations: 3,
        },
      ];

      act(() => {
        result.current.addEvents(events);
      });

      expect(result.current.taskStatus).not.toBe('idle');

      act(() => {
        result.current.clearEvents();
      });

      expect(result.current.taskStatus).toBe('idle');
      expect(result.current.toolCalls.size).toBe(0);
      expect(result.current.iterations.size).toBe(0);
      expect(result.current.currentIteration).toBe(null);
      expect(result.current.activeToolCall).toBe(null);
    });
  });

  describe('Browser Snapshot Tracking', () => {
    it('should track browser snapshots', () => {
      const { result } = renderHook(() => useAgentStreamStore());

      const snapshotEvent: AnyAgentEvent = {
        event_type: 'browser_snapshot',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        url: 'https://example.com',
        screenshot_data: 'base64-encoded-data',
        html_preview: '<html>...</html>',
      };

      act(() => {
        result.current.addEvent(snapshotEvent);
      });

      expect(result.current.browserSnapshots).toHaveLength(1);
      expect(result.current.browserSnapshots[0].url).toBe('https://example.com');
    });
  });
});
