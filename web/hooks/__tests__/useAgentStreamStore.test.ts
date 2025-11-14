import { describe, it, expect, beforeEach } from 'vitest';
import { act } from '@testing-library/react';
import { useAgentStreamStore } from '../useAgentStreamStore';
import { AnyAgentEvent } from '@/lib/types';

describe('useAgentStreamStore', () => {
  beforeEach(() => {
    act(() => {
      useAgentStreamStore.getState().clearEvents();
    });
  });

  describe('LRU Event Caching', () => {
    it('should add events to cache', () => {
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
        useAgentStreamStore.getState().addEvent(event);
      });

      const cacheUsage = useAgentStreamStore.getState().eventCache.getMemoryUsage();
      expect(cacheUsage.eventCount).toBe(1);
    });

    it('should evict oldest events when cache exceeds 1000 events', () => {
      // Add 1001 events
      const events: AnyAgentEvent[] = Array.from({ length: 1001 }, (_, i) => ({
        event_type: 'thinking',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: i,
      }));

      act(() => {
        useAgentStreamStore.getState().addEvents(events);
      });

      const cacheUsage = useAgentStreamStore.getState().eventCache.getMemoryUsage();

      // Should have exactly 1000 events (LRU eviction)
      expect(cacheUsage.eventCount).toBe(1000);
    });

    it('should maintain memory bounds under 5MB for typical events', () => {
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
        useAgentStreamStore.getState().addEvents(events);
      });

      const cacheUsage = useAgentStreamStore.getState().eventCache.getMemoryUsage();

      // Estimated memory should be under 5MB (5 * 1024 * 1024 bytes)
      expect(cacheUsage.estimatedBytes).toBeLessThan(5 * 1024 * 1024);
    });

    it('should handle event deduplication by call_id', () => {
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
        const store = useAgentStreamStore.getState();
        store.addEvent(startEvent);
        store.addEvent(streamEvent);
        store.addEvent(completeEvent);
      });

      // Should aggregate into single tool call
      const updatedState = useAgentStreamStore.getState();
      expect(updatedState.toolCalls.size).toBe(1);
      const toolCall = updatedState.toolCalls.get('call-123');
      expect(toolCall?.status).toBe('done');
      expect(toolCall?.stream_chunks).toHaveLength(1);
    });
  });

  describe('Step Tree Aggregation', () => {
    it('should track research steps correctly', () => {
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
        const store = useAgentStreamStore.getState();
        store.addEvent(planEvent);
        store.addEvent(stepStartedEvent);
        store.addEvent(stepCompletedEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.researchSteps).toHaveLength(3);
      const firstStep = state.researchSteps.find((step) => step.id === '0');
      expect(firstStep?.status).toBe('done');
      expect(firstStep?.result).toBe('Research completed successfully');
    });

    it('should group events by iteration', () => {
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
        const store = useAgentStreamStore.getState();
        store.addEvent(iterStartEvent);
        store.addEvent(thinkingEvent);
        store.addEvent(iterCompleteEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.iterations.size).toBe(1);
      const iteration = state.iterations.get(1);
      expect(iteration?.status).toBe('done');
      expect(iteration?.tokens_used).toBe(500);
    });
  });

  describe('Task Status Management', () => {
    it('should transition from idle to analyzing', () => {
      expect(useAgentStreamStore.getState().taskStatus).toBe('idle');

      const analysisEvent: AnyAgentEvent = {
        event_type: 'task_analysis',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        action_name: 'Test Task',
        goal: 'Complete the test',
        approach: 'Outline plan and execute checks',
        success_criteria: ['Pass regression tests'],
        steps: [
          {
            description: 'Review existing coverage',
            needs_external_context: true,
          },
        ],
        retrieval_plan: {
          should_retrieve: true,
          local_queries: ['coverage report'],
        },
      };

      act(() => {
        useAgentStreamStore.getState().addEvent(analysisEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.taskStatus).toBe('analyzing');
      expect(state.taskAnalysis.action_name).toBe('Test Task');
      expect(state.taskAnalysis.approach).toBe('Outline plan and execute checks');
      expect(state.taskAnalysis.success_criteria).toEqual(['Pass regression tests']);
      expect(state.taskAnalysis.steps?.[0]?.description).toBe('Review existing coverage');
      expect(state.taskAnalysis.retrieval_plan?.local_queries).toEqual(['coverage report']);
    });

    it('should transition to running on iteration start', () => {
      const iterStartEvent: AnyAgentEvent = {
        event_type: 'iteration_start',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iterations: 3,
      };

      act(() => {
        useAgentStreamStore.getState().addEvent(iterStartEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.taskStatus).toBe('running');
      expect(state.currentIteration).toBe(1);
    });

    it('should transition to completed on task complete', () => {
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
        useAgentStreamStore.getState().addEvent(completeEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.taskStatus).toBe('completed');
      expect(state.finalAnswer).toBe('Task completed successfully');
      expect(state.totalIterations).toBe(3);
      expect(state.totalTokens).toBe(1500);
    });

    it('should transition to cancelled on task cancelled', () => {
      const cancelledEvent: AnyAgentEvent = {
        event_type: 'task_cancelled',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        reason: 'cancelled',
      };

      act(() => {
        useAgentStreamStore.getState().addEvent(cancelledEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.taskStatus).toBe('cancelled');
      expect(state.errorMessage).toBeUndefined();
      expect(state.finalAnswer).toBeUndefined();
    });

    it('should transition to error on error event', () => {
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
        useAgentStreamStore.getState().addEvent(errorEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.taskStatus).toBe('error');
      expect(state.errorMessage).toBe('Something went wrong');
    });
  });

  describe('Active Tool Call Tracking', () => {
    it('should track active tool call', () => {

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
        useAgentStreamStore.getState().addEvent(startEvent);
      });

      expect(useAgentStreamStore.getState().activeToolCallId).toBe('call-456');

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
        useAgentStreamStore.getState().addEvent(completeEvent);
      });

      expect(useAgentStreamStore.getState().activeToolCallId).toBe(null);
    });
  });

  describe('Clear Events', () => {
    it('should reset all state when clearing events', () => {
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
        useAgentStreamStore.getState().addEvents(events);
      });

      expect(useAgentStreamStore.getState().taskStatus).not.toBe('idle');

      act(() => {
        useAgentStreamStore.getState().clearEvents();
      });

      const state = useAgentStreamStore.getState();
      expect(state.taskStatus).toBe('idle');
      expect(state.toolCalls.size).toBe(0);
      expect(state.iterations.size).toBe(0);
      expect(state.currentIteration).toBe(null);
      expect(state.activeToolCallId).toBe(null);
    });
  });

  describe('Browser Diagnostics Tracking', () => {
    it('should track browser diagnostics events', () => {
      const diagnosticsEvent: AnyAgentEvent = {
        event_type: 'browser_info',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        captured: new Date().toISOString(),
        success: true,
        message: 'Browser ready',
      };

      act(() => {
        useAgentStreamStore.getState().addEvent(diagnosticsEvent);
      });

      const state = useAgentStreamStore.getState();
      expect(state.browserDiagnostics).toHaveLength(1);
      expect(state.browserDiagnostics[0].success).toBe(true);
    });
  });
});
