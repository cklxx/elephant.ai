import { describe, it, expect } from 'vitest';

import { normalizeAgentEvent, normalizeAgentEvents } from '../normalize';

describe('normalizeAgentEvent', () => {
  it('coerces payload-only tool fields into the top-level event', () => {
    const raw = {
      event_type: 'workflow.tool.completed',
      timestamp: '2024-01-01T00:00:00Z',
      agent_level: 'core',
      session_id: 'session-share',
      run_id: 'task-share',
      payload: {
        tool_name: 'bash',
        call_id: 'call-1',
        result: 'ok',
        duration_ms: 123,
      },
    };

    const result = normalizeAgentEvent(raw);

    expect(result.status).toBe('valid');
    expect(result.event?.tool_name).toBe('bash');
    expect(result.event?.call_id).toBe('call-1');
    expect(result.event?.duration).toBe(123);
  });

  it('returns valid status when the event already matches schema', () => {
    const raw = {
      event_type: 'workflow.tool.completed',
      timestamp: '2024-01-02T00:00:00Z',
      agent_level: 'core',
      session_id: 'session-valid',
      call_id: 'call-2',
      tool_name: 'web_search',
      result: 'ok',
      duration: 12,
    };

    const result = normalizeAgentEvent(raw);

    expect(result.status).toBe('valid');
    expect(result.event.tool_name).toBe('web_search');
  });

  it('flags invalid events when event_type is missing', () => {
    const result = normalizeAgentEvent({ payload: { tool_name: 'bash' } });

    expect(result.status).toBe('invalid');
    expect(result.event).toBeNull();
  });

  it('dedupes duplicate workflow.result.final events per task', () => {
    const events = [
      {
        event_type: 'workflow.result.final',
        timestamp: '2024-01-01T00:00:00Z',
        agent_level: 'core',
        session_id: 'session-1',
        run_id: 'task-1',
        final_answer: 'first',
      },
      {
        event_type: 'workflow.tool.completed',
        timestamp: '2024-01-01T00:00:01Z',
        agent_level: 'core',
        session_id: 'session-1',
        call_id: 'call-1',
        tool_name: 'bash',
        result: 'ok',
        duration: 3,
      },
      {
        event_type: 'workflow.result.final',
        timestamp: '2024-01-01T00:00:02Z',
        agent_level: 'core',
        session_id: 'session-1',
        run_id: 'task-1',
        final_answer: 'second',
      },
    ];

    const normalized = normalizeAgentEvents(events);

    const finalEvents = normalized.filter((evt) => evt.event_type === 'workflow.result.final');
    expect(finalEvents).toHaveLength(1);
    expect(finalEvents[0].final_answer).toBe('second');
  });
});
