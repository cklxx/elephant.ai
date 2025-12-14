import { describe, it, expect, beforeEach } from 'vitest';
import {
  EventLRUCache,
  aggregateToolCalls,
  groupByIteration,
  extractResearchSteps,
  buildToolCallSummaries,
} from '../eventAggregation';
import { AnyAgentEvent } from '../types';

describe('EventLRUCache', () => {
  let cache: EventLRUCache;

  beforeEach(() => {
    cache = new EventLRUCache(1000);
  });

  describe('Basic Operations', () => {
    it('should add events to cache', () => {
      const event: AnyAgentEvent = {
        event_type: 'workflow.node.output.delta',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
      };

      cache.add(event);

      expect(cache.size()).toBe(1);
      expect(cache.getAll()).toContain(event);
    });

    it('should add multiple events', () => {
      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.output.delta',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
        },
        {
          event_type: 'workflow.node.output.delta',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 2,
        },
      ];

      cache.addMany(events);

      expect(cache.size()).toBe(2);
    });

    it('should clear all events', () => {
      const event: AnyAgentEvent = {
        event_type: 'workflow.node.output.delta',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
      };

      cache.add(event);
      expect(cache.size()).toBe(1);

      cache.clear();
      expect(cache.size()).toBe(0);
      expect(cache.getAll()).toEqual([]);
    });
  });

  describe('LRU Eviction', () => {
    it('should evict oldest event when exceeding max size', () => {
      const smallCache = new EventLRUCache(3);

      const events: AnyAgentEvent[] = [
        {
          event_type: 'workflow.node.output.delta',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
        },
        {
          event_type: 'workflow.node.output.delta',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 2,
        },
        {
          event_type: 'workflow.node.output.delta',
          timestamp: '2025-01-01T10:00:02Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 3,
        },
        {
          event_type: 'workflow.node.output.delta',
          timestamp: '2025-01-01T10:00:03Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 4,
        },
      ];

      smallCache.addMany(events);

      expect(smallCache.size()).toBe(3);
      const allEvents = smallCache.getAll();
      expect(allEvents[0].iteration).toBe(2); // First event evicted
      expect(allEvents[1].iteration).toBe(3);
      expect(allEvents[2].iteration).toBe(4);
    });

    it('should maintain exactly max size when adding many events', () => {
      const events: AnyAgentEvent[] = Array.from({ length: 1500 }, (_, i) => ({
        event_type: 'workflow.node.output.delta',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: i,
      }));

      cache.addMany(events);

      expect(cache.size()).toBe(1000);
      // Should keep the last 1000 events
      const allEvents = cache.getAll();
      expect(allEvents[0].iteration).toBe(500);
      expect(allEvents[999].iteration).toBe(1499);
    });
  });

  describe('Memory Usage', () => {
    it('should estimate memory usage', () => {
      const events: AnyAgentEvent[] = Array.from({ length: 100 }, (_, i) => ({
        event_type: 'workflow.node.output.delta',
        timestamp: new Date().toISOString(),
        session_id: 'test-123',
        agent_level: 'core',
        iteration: i,
      }));

      cache.addMany(events);

      const usage = cache.getMemoryUsage();
      expect(usage.eventCount).toBe(100);
      expect(usage.estimatedBytes).toBe(100 * 500); // 500 bytes per event
    });

    it('should replace only the last matching event (adjacent streaming updates)', () => {
      const first: AnyAgentEvent = {
        event_type: 'workflow.result.final',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'session-1',
        task_id: 'task-1',
        agent_level: 'core',
        final_answer: 'first answer',
        total_iterations: 1,
        total_tokens: 10,
        stop_reason: 'final_answer',
        duration: 1000,
      };
      const second: AnyAgentEvent = {
        ...first,
        timestamp: '2025-01-01T10:00:01Z',
        final_answer: 'streaming update',
      };

      cache.add(first);
      const replaced = cache.replaceLastIf(
        (event) => event.event_type === 'workflow.result.final' && event.task_id === 'task-1',
        second,
      );
      if (!replaced) {
        cache.add(second);
      }

      expect(replaced).toBe(true);
      const all = cache.getAll();
      expect(all).toHaveLength(1);
      expect(all[0].final_answer).toBe('streaming update');
    });

    it('should not replace non-adjacent matching events', () => {
      const first: AnyAgentEvent = {
        event_type: 'workflow.result.final',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'session-1',
        task_id: 'task-1',
        agent_level: 'core',
        final_answer: 'first answer',
        total_iterations: 1,
        total_tokens: 10,
        stop_reason: 'final_answer',
        duration: 1000,
      };
      const deltaEvent: AnyAgentEvent = {
        event_type: 'workflow.node.output.delta',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'session-1',
        agent_level: 'core',
        iteration: 1,
        delta: '...',
      };
      const second: AnyAgentEvent = {
        ...first,
        timestamp: '2025-01-01T10:00:02Z',
        final_answer: 'streaming update',
      };

      cache.add(first);
      cache.add(deltaEvent);
      const replaced = cache.replaceLastIf(
        (event) => event.event_type === 'workflow.result.final' && event.task_id === 'task-1',
        second,
      );

      expect(replaced).toBe(false);
      if (!replaced) {
        cache.add(second);
      }
      const all = cache.getAll();
      expect(all).toHaveLength(3);
      expect(all[0].final_answer).toBe('first answer');
      expect(all[2].final_answer).toBe('streaming update');
    });
  });
});

describe('aggregateToolCalls', () => {
  it('should aggregate tool call start events', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'ls' },
      },
    ];

    const result = aggregateToolCalls(events);

    expect(result.size).toBe(1);
    const toolCall = result.get('call-1');
    expect(toolCall).toMatchObject({
      call_id: 'call-1',
      tool_name: 'bash',
      arguments: { command: 'ls' },
      status: 'running',
      stream_chunks: [],
    });
  });

  it('should aggregate streaming chunks', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'ls' },
      },
      {
        event_type: 'workflow.tool.progress',
        timestamp: '2025-01-01T10:00:01Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        chunk: 'file1.txt\n',
      },
      {
        event_type: 'workflow.tool.progress',
        timestamp: '2025-01-01T10:00:02Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        chunk: 'file2.txt\n',
      },
    ];

    const result = aggregateToolCalls(events);

    const toolCall = result.get('call-1');
    expect(toolCall?.status).toBe('streaming');
    expect(toolCall?.stream_chunks).toEqual(['file1.txt\n', 'file2.txt\n']);
    expect(toolCall?.last_stream_at).toBe('2025-01-01T10:00:02Z');
  });

  it('should complete tool calls', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'ls' },
      },
      {
        event_type: 'workflow.tool.completed',
        timestamp: '2025-01-01T10:00:05Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        result: 'file1.txt\nfile2.txt',
        duration: 5000,
      },
    ];

    const result = aggregateToolCalls(events);

    const toolCall = result.get('call-1');
    expect(toolCall?.status).toBe('complete');
    expect(toolCall?.result).toBe('file1.txt\nfile2.txt');
    expect(toolCall?.duration).toBe(5000);
    expect(toolCall?.completed_at).toBe('2025-01-01T10:00:05Z');
  });

  it('should handle tool call errors', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'invalid' },
      },
      {
        event_type: 'workflow.tool.completed',
        timestamp: '2025-01-01T10:00:01Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        result: '',
        error: 'Command not found',
        duration: 1000,
      },
    ];

    const result = aggregateToolCalls(events);

    const toolCall = result.get('call-1');
    expect(toolCall?.status).toBe('error');
    expect(toolCall?.error).toBe('Command not found');
    expect(toolCall?.completed_at).toBe('2025-01-01T10:00:01Z');
  });

  it('should handle complete without start (orphaned complete)', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.completed',
        timestamp: '2025-01-01T10:00:01Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'orphan-call',
        tool_name: 'bash',
        result: 'orphan result',
        duration: 1000,
      },
    ];

    const result = aggregateToolCalls(events);

    const toolCall = result.get('orphan-call');
    expect(toolCall).toBeDefined();
    expect(toolCall?.status).toBe('complete');
    expect(toolCall?.result).toBe('orphan result');
    expect(toolCall?.arguments).toEqual({});
    expect(toolCall?.completed_at).toBe('2025-01-01T10:00:01Z');
  });
});

describe('buildToolCallSummaries', () => {
  it('creates tool call summaries', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'code_execute',
        arguments: { language: 'python', source: 'print("hi")' },
      },
      {
        event_type: 'workflow.tool.completed',
        timestamp: '2025-01-01T10:00:02Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'code_execute',
        result: 'hi\n',
        duration: 2000,
      },
    ];

    const summaries = buildToolCallSummaries(events);
    expect(summaries).toHaveLength(1);
    const summary = summaries[0];
    expect(summary.status).toBe('completed');
    expect(summary.durationMs).toBe(2000);
    expect(summary.resultPreview).toContain('hi');
    expect(summary.argumentsPreview).toContain('language');
  });

  it('marks running tools without completion', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-2',
        tool_name: 'search_docs',
        arguments: { query: 'example' },
      },
    ];

    const summaries = buildToolCallSummaries(events);
    expect(summaries).toHaveLength(1);
    const summary = summaries[0];
    expect(summary.status).toBe('running');
    expect(summary.completedAt).toBeUndefined();
  });

  it('orders summaries by start timestamp', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-3',
        tool_name: 'shell_exec',
        arguments: { command: 'ls' },
      },
      {
        event_type: 'workflow.tool.completed',
        timestamp: '2025-01-01T10:00:03Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-3',
        tool_name: 'shell_exec',
        result: 'file.txt',
        duration: 3000,
      },
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T11:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 2,
        call_id: 'call-4',
        tool_name: 'file_write',
        arguments: { path: 'notes.md' },
      },
    ];

    const summaries = buildToolCallSummaries(events);
    expect(summaries).toHaveLength(2);
    expect(summaries[0].callId).toBe('call-3');
    expect(summaries[0].toolName).toBe('shell_exec');
    expect(summaries[1].callId).toBe('call-4');
    expect(summaries[1].toolName).toBe('file_write');
  });
});

describe('groupByIteration', () => {
  it('should create iteration groups from workflow.node.started events', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iters: 5,
      },
    ];

    const result = groupByIteration(events);

    expect(result.size).toBe(1);
    const group = result.get(1);
    expect(group).toMatchObject({
      id: 'iter-1',
      iteration: 1,
      total_iters: 5,
      status: 'running',
      tool_calls: [],
      errors: [],
    });
  });

  it('should complete iterations', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iters: 5,
      },
      {
        event_type: 'workflow.node.completed',
        timestamp: '2025-01-01T10:01:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        tokens_used: 500,
        tools_run: 2,
      },
    ];

    const result = groupByIteration(events);

    const group = result.get(1);
    expect(group?.status).toBe('complete');
    expect(group?.tokens_used).toBe(500);
    expect(group?.tools_run).toBe(2);
  });

  it('should add workflow.node.output.delta to iterations', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iters: 5,
      },
      {
        event_type: 'workflow.node.output.summary',
        timestamp: '2025-01-01T10:00:30Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        content: 'I need to analyze the code',
      },
    ];

    const result = groupByIteration(events);

    const group = result.get(1);
    expect(group?.delta).toBe('I need to analyze the code');
  });

  it('should group tool calls by iteration', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iters: 3,
      },
      {
        event_type: 'workflow.tool.started',
        timestamp: '2025-01-01T10:00:10Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'ls' },
      },
      {
        event_type: 'workflow.tool.completed',
        timestamp: '2025-01-01T10:00:11Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        result: 'output',
        duration: 1000,
      },
    ];

    const result = groupByIteration(events);

    const group = result.get(1);
    expect(group?.tool_calls).toHaveLength(1);
    expect(group?.tool_calls[0].call_id).toBe('call-1');
  });

  it('should track errors in iterations', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iters: 3,
      },
      {
        event_type: 'workflow.node.failed',
        timestamp: '2025-01-01T10:00:30Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        phase: 'execute',
        error: 'Tool execution failed',
        recoverable: false,
      },
    ];

    const result = groupByIteration(events);

    const group = result.get(1);
    expect(group?.errors).toContain('Tool execution failed');
  });
});

describe('extractResearchSteps', () => {
  it('should update step status on step_started', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:30Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        step_index: 0,
        step_description: 'Step 1: Analyze',
      },
    ];

    const result = extractResearchSteps(events);

    expect(result[0].status).toBe('in_progress');
    expect(result[0].started_at).toBe('2025-01-01T10:00:30Z');
  });

  it('should update step status on step_completed', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:30Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        step_index: 0,
        step_description: 'Step 1: Analyze',
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

    const result = extractResearchSteps(events);

    expect(result[0].status).toBe('completed');
    expect(result[0].completed_at).toBe('2025-01-01T10:05:00Z');
    expect(result[0].result).toBe('Analysis complete');
  });

  it('should handle no research plan', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'workflow.node.started',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iters: 3,
      },
    ];

    const result = extractResearchSteps(events);

    expect(result).toEqual([]);
  });
});
