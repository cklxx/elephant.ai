import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { useTimelineSteps } from '../useTimelineSteps';
import { AnyAgentEvent } from '@/lib/types';

describe('useTimelineSteps', () => {
  describe('Step Index Events', () => {
    it('should create steps from step_started events', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.started',
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
        title: 'Analyze the codebase',
        status: 'active',
      });
    });

    it('should complete steps when step_completed events arrive', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Analyze the codebase',
        },
        {
          event_type: 'workflow.node.completed',
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
        title: 'Analyze the codebase',
        status: 'done',
      });
      expect(result.current[0].duration).toBe(5 * 60 * 1000); // 5 minutes
    });

    it('should handle multiple steps in sequence', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Step 1',
        },
        {
          event_type: 'workflow.node.completed',
          timestamp: '2025-01-01T10:05:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 2,
          step_index: 0,
          step_result: 'Step 1 done',
        },
        {
          event_type: 'workflow.node.started',
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
      expect(result.current[0].status).toBe('done');
      expect(result.current[1].status).toBe('active');
    });
  });

  describe('Error Handling', () => {
    it('should mark steps as failed when workflow.node.completed has failed status', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          step_index: 0,
          step_description: 'Run tool',
        },
        {
          event_type: 'workflow.node.completed',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          step_index: 0,
          step_description: 'Run tool',
          status: 'failed',
          step_result: { error: 'Tool execution failed' },
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(1);
      expect(result.current[0]).toMatchObject({
        id: 'step-0',
        title: 'Run tool',
        status: 'failed',
      });
    });
  });

  describe('Step Ordering', () => {
    it('should sort steps by start time', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:02:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 1,
          step_description: 'Second',
        },
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'First',
        },
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:04:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 2,
          step_description: 'Third',
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      expect(result.current).toHaveLength(3);
      expect(result.current[0].id).toBe('step-0');
      expect(result.current[1].id).toBe('step-1');
      expect(result.current[2].id).toBe('step-2');
    });
  });

  describe('Mixed Step Types', () => {
    it('should handle both research steps and iteration steps', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Research step',
        },
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          total_iters: 3,
        },
        {
          event_type: 'workflow.node.completed',
          timestamp: '2025-01-01T10:02:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          tokens_used: 300,
          tools_run: 1,
        },
      ];

      const { result } = renderHook(() => useTimelineSteps(events));

      // Plan steps should take precedence over fallback iteration entries
      expect(result.current).toHaveLength(1);
      expect(result.current[0].id).toBe('step-0');
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
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Work',
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
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Work',
        },
      ];

      const events2: AnyAgentEvent[] = [
        ...events1,
        {
          event_type: 'workflow.node.completed',
          timestamp: '2025-01-01T10:01:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
          step_index: 0,
          step_description: 'Work',
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
      expect(result.current[0].status).toBe('done');
    });
  });

  describe('Stage Titles', () => {
    it('should replace prepare/execute stage titles when no plan exists', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.input.received',
          timestamp: '2025-01-01T09:59:59Z',
          session_id: 'test-123',
          agent_level: 'core',
          task: '把 ConversationEventStream 改名，并让 UI 更像 Manus',
        } as AnyAgentEvent,
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          step_index: 0,
          step_description: 'prepare',
        } as AnyAgentEvent,
        {
          event_type: 'workflow.node.completed',
          timestamp: '2025-01-01T10:00:02Z',
          session_id: 'test-123',
          agent_level: 'core',
          step_index: 0,
          step_description: 'prepare',
          step_result: { approach: '先抽出组件，再统一进度展示' },
        } as AnyAgentEvent,
        {
          event_type: 'workflow.node.started',
          timestamp: '2025-01-01T10:00:03Z',
          session_id: 'test-123',
          agent_level: 'core',
          step_index: 1,
          step_description: 'execute',
        } as AnyAgentEvent,
      ];

      const { result } = renderHook(() => useTimelineSteps(events));
      expect(result.current).toHaveLength(2);
      expect(result.current[0].title).toBe('先抽出组件，再统一进度展示');
      expect(result.current[1].title).toContain('把 ConversationEventStream 改名');
    });

    it('should return empty when there are no explicit steps', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.tool.completed',
          timestamp: '2025-01-01T09:59:59Z',
          session_id: 'test-123',
          agent_level: 'core',
          run_id: 'task-1',
          call_id: 'call-plan',
          tool_name: 'plan',
          result: '做一次无步骤的执行。',
          duration: 1,
        } as AnyAgentEvent,
      ];

      const { result } = renderHook(() => useTimelineSteps(events));
      expect(result.current).toHaveLength(0);
    });
  });
});
