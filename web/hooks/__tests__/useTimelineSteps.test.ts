import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useTimelineSteps } from '../useTimelineSteps';
import { AnyAgentEvent } from '@/lib/types';

describe('useTimelineSteps', () => {
  describe('Research Plan Steps', () => {
    it('should create steps from step_started events', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'step_started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Analyze the codebase',
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'step-0',
        title: 'Step 1',
        description: 'Analyze the codebase',
        status: 'active',
      });
    });

    it('should complete steps when step_completed events arrive', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'step_started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Analyze the codebase',
        },
        {
          event_type: 'step_completed',
          timestamp: '2025-01-01T10:05:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 2,
          step_index: 0,
          step_result: 'Analysis complete',
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'step-0',
        title: 'Step 1',
        description: 'Analyze the codebase',
        status: 'complete',
      });
      expect(result.current[0].duration).toBe(5 * 60 * 1000); // 5 minutes
    });

    it('should handle multiple steps in sequence', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'step_started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Step 1',
        },
        {
          event_type: 'step_completed',
          timestamp: '2025-01-01T10:05:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 2,
          step_index: 0,
          step_result: 'Step 1 done',
        },
        {
          event_type: 'step_started',
          timestamp: '2025-01-01T10:05:30Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 3,
          step_index: 1,
          step_description: 'Step 2',
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(2);
      expect(result.current[0].status).toBe('complete');
      expect(result.current[1].status).toBe('active');
    });
  });

  describe('Iteration-based Steps (Fallback)', () => {
    it('should create steps from iteration_start events', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 5,
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'iteration-1',
        title: 'Iteration 1/5',
        status: 'active',
      });
    });

    it('should complete iterations when iteration_complete events arrive', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 5,
        },
        {
          event_type: 'iteration_complete',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          tokens_used: 500,
          tools_run: 2,
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'iteration-1',
        title: 'Iteration 1/5',
        status: 'complete',
        tokensUsed: 500,
      });
      expect(result.current[0].duration).toBe(60 * 1000); // 1 minute
    });

    it('should track tools used in iterations', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 3,
        },
        {
          event_type: 'tool_call_start',
          timestamp: '2025-01-01T10:00:10Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-1',
          tool_name: 'file_read',
          arguments: { path: '/test.txt' },
        },
        {
          event_type: 'tool_call_start',
          timestamp: '2025-01-01T10:00:20Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          call_id: 'call-2',
          tool_name: 'bash',
          arguments: { command: 'ls' },
        },
        {
          event_type: 'iteration_complete',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          tokens_used: 500,
          tools_run: 2,
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0].toolsUsed).toEqual(['file_read', 'bash']);
    });
  });

  describe('Error Handling', () => {
    it('should mark iterations as error when error events occur', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 5,
        },
        {
          event_type: 'error',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          phase: 'execute',
          error: 'Tool execution failed',
          recoverable: false,
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'iteration-1',
        title: 'Iteration 1/5',
        status: 'error',
        error: 'Tool execution failed',
      });
    });
  });

  describe('Step Ordering', () => {
    it('should sort steps by start time', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:02:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 2,
          total_iters: 3,
        },
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 3,
        },
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:04:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 3,
          total_iters: 3,
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(3);
      expect(result.current[0].id).toBe('iteration-1');
      expect(result.current[1].id).toBe('iteration-2');
      expect(result.current[2].id).toBe('iteration-3');
    });
  });

  describe('Mixed Step Types', () => {
    it('should handle both research steps and iteration steps', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'step_started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Research step',
        },
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 3,
        },
        {
          event_type: 'iteration_complete',
          timestamp: '2025-01-01T10:02:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          tokens_used: 300,
          tools_run: 1,
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      // Should have both research step (active) and iteration step (complete)
      expect(result.current).toHaveLength(2);
      expect(result.current.some(s => s.id === 'step-0')).toBe(true);
      expect(result.current.some(s => s.id === 'iteration-1')).toBe(true);
    });
  });

  describe('Empty Events', () => {
    it('should return empty array for no events', () => {
      const { result } = renderHook(() => useTimelineSteps([]));

      expect(result.current).toEqual([]);
    });
  });

  describe('Memoization', () => {
    it('should memoize results when events do not change', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 3,
        },
      ];

      const { result, rerender } = renderHook(
        ({ events }) => useTimelineSteps(events),
        { initialProps: { events } }
      );

      const firstResult = result.current;

      rerender({ events }); // Same events

      expect(result.current).toBe(firstResult); // Should be same reference
    });

    it('should recompute when events change', () => {
      const events1: AnyAgentEvent[] = [
        {
          event_type: 'iteration_start',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 3,
        },
      ];

      const events2: AnyAgentEvent[] = [
        ...events1,
        {
          event_type: 'iteration_complete',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          tokens_used: 300,
          tools_run: 1,
        },
      ];

      const { result, rerender } = renderHook(
        ({ events }) => useTimelineSteps(events),
        { initialProps: { events: events1 } }
      );

      const firstResult = result.current;
      expect(firstResult).toHaveLength(1);
      expect(firstResult[0].status).toBe('active');

      rerender({ events: events2 });

      expect(result.current).not.toBe(firstResult); // Should be new reference
      expect(result.current).toHaveLength(1);
      expect(result.current[0].status).toBe('complete');
    });
  });
});
