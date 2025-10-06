import { describe, it, expect, beforeEach } from 'vitest';
import {
  EventLRUCache,
  aggregateToolCalls,
  groupByIteration,
  extractResearchSteps,
  extractBrowserSnapshots,
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
        event_type: 'thinking',
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
          event_type: 'thinking',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
        },
        {
          event_type: 'thinking',
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
        event_type: 'thinking',
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
          event_type: 'thinking',
          timestamp: '2025-01-01T10:00:00Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 1,
        },
        {
          event_type: 'thinking',
          timestamp: '2025-01-01T10:00:01Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 2,
        },
        {
          event_type: 'thinking',
          timestamp: '2025-01-01T10:00:02Z',
          session_id: 'test-123',
          agent_level: 'core',
          iteration: 3,
        },
        {
          event_type: 'thinking',
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
        event_type: 'thinking',
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
        event_type: 'thinking',
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
  });
});

describe('aggregateToolCalls', () => {
  it('should aggregate tool call start events', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'tool_call_start',
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
        event_type: 'tool_call_start',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'ls' },
      },
      {
        event_type: 'tool_call_stream',
        timestamp: '2025-01-01T10:00:01Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        chunk: 'file1.txt\n',
      },
      {
        event_type: 'tool_call_stream',
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
  });

  it('should complete tool calls', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'tool_call_start',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'ls' },
      },
      {
        event_type: 'tool_call_complete',
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
  });

  it('should handle tool call errors', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'tool_call_start',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        call_id: 'call-1',
        tool_name: 'bash',
        arguments: { command: 'invalid' },
      },
      {
        event_type: 'tool_call_complete',
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
  });

  it('should handle complete without start (orphaned complete)', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'tool_call_complete',
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
  });
});

describe('groupByIteration', () => {
  it('should create iteration groups from iteration_start events', () => {
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

    const result = groupByIteration(events);

    const group = result.get(1);
    expect(group?.status).toBe('complete');
    expect(group?.tokens_used).toBe(500);
    expect(group?.tools_run).toBe(2);
  });

  it('should add thinking to iterations', () => {
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
        event_type: 'think_complete',
        timestamp: '2025-01-01T10:00:30Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        content: 'I need to analyze the code',
      },
    ];

    const result = groupByIteration(events);

    const group = result.get(1);
    expect(group?.thinking).toBe('I need to analyze the code');
  });

  it('should group tool calls by iteration', () => {
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
        tool_name: 'bash',
        arguments: { command: 'ls' },
      },
      {
        event_type: 'tool_call_complete',
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
        event_type: 'iteration_start',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        total_iters: 3,
      },
      {
        event_type: 'error',
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
  it('should extract steps from research_plan event', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'research_plan',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        plan_steps: ['Step 1: Analyze', 'Step 2: Implement', 'Step 3: Test'],
        estimated_iterations: 5,
      },
    ];

    const result = extractResearchSteps(events);

    expect(result).toHaveLength(3);
    expect(result[0]).toMatchObject({
      id: 'step-0',
      step_index: 0,
      description: 'Step 1: Analyze',
      status: 'pending',
    });
  });

  it('should update step status on step_started', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'research_plan',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        plan_steps: ['Step 1: Analyze'],
        estimated_iterations: 3,
      },
      {
        event_type: 'step_started',
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
        event_type: 'research_plan',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        plan_steps: ['Step 1: Analyze'],
        estimated_iterations: 3,
      },
      {
        event_type: 'step_started',
        timestamp: '2025-01-01T10:00:30Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        step_index: 0,
        step_description: 'Step 1: Analyze',
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

    const result = extractResearchSteps(events);

    expect(result[0].status).toBe('completed');
    expect(result[0].completed_at).toBe('2025-01-01T10:05:00Z');
    expect(result[0].result).toBe('Analysis complete');
  });

  it('should handle no research plan', () => {
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

    const result = extractResearchSteps(events);

    expect(result).toEqual([]);
  });
});

describe('extractBrowserSnapshots', () => {
  it('should extract browser snapshots', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'browser_snapshot',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        url: 'https://example.com',
        screenshot_data: 'base64-data',
        html_preview: '<html>...</html>',
      },
    ];

    const result = extractBrowserSnapshots(events);

    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({
      url: 'https://example.com',
      screenshot_data: 'base64-data',
      html_preview: '<html>...</html>',
    });
  });

  it('should filter out non-snapshot events', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'thinking',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
      },
      {
        event_type: 'browser_snapshot',
        timestamp: '2025-01-01T10:00:01Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
        url: 'https://example.com',
      },
    ];

    const result = extractBrowserSnapshots(events);

    expect(result).toHaveLength(1);
    expect(result[0].url).toBe('https://example.com');
  });

  it('should handle no snapshots', () => {
    const events: AnyAgentEvent[] = [
      {
        event_type: 'thinking',
        timestamp: '2025-01-01T10:00:00Z',
        session_id: 'test-123',
        agent_level: 'core',
        iteration: 1,
      },
    ];

    const result = extractBrowserSnapshots(events);

    expect(result).toEqual([]);
  });
});
