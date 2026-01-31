import { describe, expect, it } from 'vitest';
import {
  sortEventsBySeq,
  partitionEvents,
  parseEventTimestamp,
  getEventKey,
  isSubagentToolStarted,
  isSubagentToolEvent,
} from '../eventStreamUtils';
import type { AnyAgentEvent } from '@/lib/types';

function makeEvent(overrides: Partial<AnyAgentEvent> & { event_type: string }): AnyAgentEvent {
  return {
    session_id: 'session-1',
    run_id: 'run-1',
    agent_level: 'core',
    timestamp: new Date().toISOString(),
    ...overrides,
  } as AnyAgentEvent;
}

describe('sortEventsBySeq', () => {
  it('sorts events by seq number when present', () => {
    const events = [
      makeEvent({ event_type: 'workflow.input.received', seq: 3 }),
      makeEvent({ event_type: 'workflow.input.received', seq: 1 }),
      makeEvent({ event_type: 'workflow.input.received', seq: 2 }),
    ];

    const sorted = sortEventsBySeq(events);
    expect((sorted[0] as any).seq).toBe(1);
    expect((sorted[1] as any).seq).toBe(2);
    expect((sorted[2] as any).seq).toBe(3);
  });

  it('treats seq=0 as unassigned and falls back to timestamp', () => {
    const events = [
      makeEvent({ event_type: 'workflow.input.received', seq: 0, timestamp: '2026-01-30T12:00:00Z' }),
      makeEvent({ event_type: 'workflow.tool.started', seq: 1, timestamp: '2026-01-30T12:00:01Z' }),
    ];

    const sorted = sortEventsBySeq(events);
    // seq=0 is treated as "no seq", so timestamp ordering is used;
    // input (T+0) comes before tool.started (T+1)
    expect(sorted[0].event_type).toBe('workflow.input.received');
    expect(sorted[1].event_type).toBe('workflow.tool.started');
  });

  it('places events without seq before same-timestamp events with seq', () => {
    const ts = '2026-01-30T12:00:00Z';
    const events = [
      makeEvent({ event_type: 'workflow.tool.started', seq: 1, timestamp: ts }),
      makeEvent({ event_type: 'workflow.input.received', timestamp: ts }),
    ];

    const sorted = sortEventsBySeq(events);
    // Same timestamp: event without seq (input) precedes event with seq
    expect(sorted[0].event_type).toBe('workflow.input.received');
    expect(sorted[1].event_type).toBe('workflow.tool.started');
  });

  it('orders multi-turn events chronologically by timestamp', () => {
    const events = [
      makeEvent({ event_type: 'workflow.input.received', seq: 0, timestamp: '2026-01-30T12:00:00Z' }),
      makeEvent({ event_type: 'workflow.tool.started', seq: 1, timestamp: '2026-01-30T12:00:01Z' }),
      makeEvent({ event_type: 'workflow.result.final', seq: 2, timestamp: '2026-01-30T12:00:02Z' }),
      makeEvent({ event_type: 'workflow.input.received', seq: 0, timestamp: '2026-01-30T12:00:03Z' }),
      makeEvent({ event_type: 'workflow.tool.started', seq: 1, timestamp: '2026-01-30T12:00:04Z' }),
      makeEvent({ event_type: 'workflow.result.final', seq: 2, timestamp: '2026-01-30T12:00:05Z' }),
    ];

    const sorted = sortEventsBySeq(events);
    // All events should maintain chronological order across turns
    expect(sorted.map(e => e.event_type)).toEqual([
      'workflow.input.received',
      'workflow.tool.started',
      'workflow.result.final',
      'workflow.input.received',
      'workflow.tool.started',
      'workflow.result.final',
    ]);
  });

  it('falls back to timestamp when no seq', () => {
    const events = [
      makeEvent({ event_type: 'workflow.input.received', timestamp: '2026-01-30T12:00:02Z' }),
      makeEvent({ event_type: 'workflow.input.received', timestamp: '2026-01-30T12:00:00Z' }),
      makeEvent({ event_type: 'workflow.input.received', timestamp: '2026-01-30T12:00:01Z' }),
    ];

    const sorted = sortEventsBySeq(events);
    expect(sorted[0].timestamp).toBe('2026-01-30T12:00:00Z');
    expect(sorted[1].timestamp).toBe('2026-01-30T12:00:01Z');
    expect(sorted[2].timestamp).toBe('2026-01-30T12:00:02Z');
  });

  it('preserves insertion order for equal seq', () => {
    const a = makeEvent({ event_type: 'workflow.input.received', seq: 1 });
    const b = makeEvent({ event_type: 'workflow.tool.started', seq: 1 });
    const events = [a, b];

    const sorted = sortEventsBySeq(events);
    expect(sorted[0]).toBe(a);
    expect(sorted[1]).toBe(b);
  });

  it('handles empty array', () => {
    expect(sortEventsBySeq([])).toEqual([]);
  });
});

describe('partitionEvents', () => {
  it('separates subagent events into groups by parent_run_id', () => {
    const events: AnyAgentEvent[] = [
      makeEvent({ event_type: 'workflow.input.received', task: 'test' }),
      makeEvent({
        event_type: 'workflow.tool.started',
        tool_name: 'subagent',
        call_id: 'sub-call-1',
        run_id: 'parent-1',
      }),
      makeEvent({
        event_type: 'workflow.tool.completed',
        tool_name: 'web_search',
        call_id: 'inner-call-1',
        run_id: 'sub-run-1',
        parent_run_id: 'parent-1',
        agent_level: 'subagent',
        subtask_index: 0,
        result: 'done',
      }),
    ];

    const result = partitionEvents(events, false);

    // Main stream should have input + subagent tool started
    expect(result.mainStream).toHaveLength(2);
    expect(result.mainStream[0].event_type).toBe('workflow.input.received');
    expect(result.mainStream[1].event_type).toBe('workflow.tool.started');

    // Subagent group by parent_run_id
    expect(result.subagentGroups.has('parent-1')).toBe(true);
  });

  it('tracks pending tools (started but not completed)', () => {
    const events: AnyAgentEvent[] = [
      makeEvent({
        event_type: 'workflow.tool.started',
        tool_name: 'web_search',
        call_id: 'call-1',
        arguments: { query: 'test' },
      }),
    ];

    const result = partitionEvents(events, true);
    expect(result.pendingTools.size).toBe(1);
    expect(result.pendingTools.has('session-1:call-1')).toBe(true);
  });

  it('groups subagent events only by parent_run_id', () => {
    const events: AnyAgentEvent[] = [
      makeEvent({
        event_type: 'workflow.tool.completed',
        tool_name: 'web_search',
        call_id: 'inner-call-1',
        run_id: 'sub-run-1',
        parent_run_id: 'parent-1',
        causation_id: 'call-123',
        agent_level: 'subagent',
        result: 'done',
      }),
    ];

    const result = partitionEvents(events, false);
    expect(Array.from(result.subagentGroups.keys())).toEqual(['parent-1']);
  });

  it('resolves paired tool start for completed events', () => {
    const started = makeEvent({
      event_type: 'workflow.tool.started',
      tool_name: 'web_search',
      call_id: 'call-1',
      arguments: { query: 'test' },
    });
    const completed = makeEvent({
      event_type: 'workflow.tool.completed',
      tool_name: 'web_search',
      call_id: 'call-1',
      result: 'search results',
    });

    const result = partitionEvents([started, completed], false);
    const paired = result.resolvePairedToolStart(completed);

    expect(paired).toBe(started);
    // Completed tool should not be in pending
    expect(result.pendingTools.size).toBe(0);
  });

  it('excludes orchestrator tools (plan, clarify) from pending', () => {
    const events: AnyAgentEvent[] = [
      makeEvent({
        event_type: 'workflow.tool.started',
        tool_name: 'plan',
        call_id: 'plan-call',
      }),
      makeEvent({
        event_type: 'workflow.tool.started',
        tool_name: 'clarify',
        call_id: 'clarify-call',
      }),
      makeEvent({
        event_type: 'workflow.tool.started',
        tool_name: 'web_search',
        call_id: 'search-call',
      }),
    ];

    const result = partitionEvents(events, true);
    expect(result.pendingTools.size).toBe(1);
    expect(result.pendingTools.has('session-1:search-call')).toBe(true);
  });

  it('filters workflow.result.final with stop_reason=await_user_input from main stream', () => {
    const events: AnyAgentEvent[] = [
      makeEvent({ event_type: 'workflow.input.received', task: 'test' }),
      makeEvent({
        event_type: 'workflow.result.final',
        stop_reason: 'await_user_input',
        final_answer: '',
        total_iterations: 1,
        total_tokens: 0,
        duration: 100,
      }),
      makeEvent({ event_type: 'workflow.input.received', task: 'follow-up' }),
      makeEvent({
        event_type: 'workflow.result.final',
        stop_reason: 'final_answer',
        final_answer: 'done',
        total_iterations: 2,
        total_tokens: 100,
        duration: 200,
      }),
    ];

    const result = partitionEvents(events, false);
    // await_user_input should be filtered; only inputs + real final remain
    expect(result.mainStream).toHaveLength(3);
    expect(result.mainStream[0].event_type).toBe('workflow.input.received');
    expect(result.mainStream[1].event_type).toBe('workflow.input.received');
    expect(result.mainStream[2].event_type).toBe('workflow.result.final');
    expect((result.mainStream[2] as any).stop_reason).toBe('final_answer');
  });

  it('filters workflow.node.started/completed from main stream', () => {
    const events: AnyAgentEvent[] = [
      makeEvent({ event_type: 'workflow.input.received', task: 'test' }),
      makeEvent({ event_type: 'workflow.node.started', node_id: 'n1' } as AnyAgentEvent),
      makeEvent({ event_type: 'workflow.node.completed', node_id: 'n1' } as AnyAgentEvent),
    ];

    const result = partitionEvents(events, false);
    expect(result.mainStream).toHaveLength(1);
    expect(result.mainStream[0].event_type).toBe('workflow.input.received');
  });
});

describe('parseEventTimestamp', () => {
  it('parses valid ISO timestamp', () => {
    const event = makeEvent({ event_type: 'test', timestamp: '2026-01-30T12:00:00Z' });
    const ts = parseEventTimestamp(event);
    expect(ts).toBe(Date.parse('2026-01-30T12:00:00Z'));
  });

  it('returns null for invalid timestamp', () => {
    const event = makeEvent({ event_type: 'test', timestamp: 'not-a-date' });
    expect(parseEventTimestamp(event)).toBeNull();
  });

  it('returns null for missing timestamp', () => {
    const event = makeEvent({ event_type: 'test', timestamp: undefined });
    expect(parseEventTimestamp(event)).toBeNull();
  });
});

describe('getEventKey', () => {
  it('uses event_id when available', () => {
    const event = makeEvent({ event_type: 'test', event_id: 'evt-123' });
    expect(getEventKey(event, 0)).toBe('event-evt-123');
  });

  it('falls back to seq when no event_id', () => {
    const event = makeEvent({ event_type: 'test', seq: 42 });
    expect(getEventKey(event, 5)).toBe('event-seq-42-5');
  });

  it('falls back to event_type and index', () => {
    const event = makeEvent({ event_type: 'workflow.input.received' });
    expect(getEventKey(event, 3)).toBe('event-workflow.input.received-3');
  });
});

describe('isSubagentToolStarted', () => {
  it('returns true for subagent tool.started events', () => {
    const event = makeEvent({
      event_type: 'workflow.tool.started',
      tool_name: 'subagent',
      call_id: 'c1',
    });
    expect(isSubagentToolStarted(event)).toBe(true);
  });

  it('returns false for non-subagent tool.started events', () => {
    const event = makeEvent({
      event_type: 'workflow.tool.started',
      tool_name: 'web_search',
      call_id: 'c1',
    });
    expect(isSubagentToolStarted(event)).toBe(false);
  });
});

describe('isSubagentToolEvent', () => {
  it('returns false for wrong event type', () => {
    const event = makeEvent({
      event_type: 'workflow.input.received',
      tool_name: 'subagent',
    });
    expect(isSubagentToolEvent(event, 'workflow.tool.started')).toBe(false);
  });

  it('handles case-insensitive tool name matching', () => {
    const event = makeEvent({
      event_type: 'workflow.tool.started',
      tool_name: 'SubAgent',
      call_id: 'c1',
    });
    expect(isSubagentToolEvent(event, 'workflow.tool.started')).toBe(true);
  });
});
