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

  it('accepts workflow.replan.requested events', () => {
    const raw = {
      event_type: 'workflow.replan.requested',
      timestamp: '2024-01-02T00:00:00Z',
      agent_level: 'core',
      session_id: 'session-replan',
      reason: 'orchestrator tool failure triggered replan injection',
      error: 'boom',
    };

    const result = normalizeAgentEvent(raw);

    expect(result.status).toBe('valid');
    expect(result.event?.event_type).toBe('workflow.replan.requested');
  });

  it('accepts workflow.tool.completed events with tool_sla payload', () => {
    const raw = {
      event_type: 'workflow.tool.completed',
      timestamp: '2024-01-02T00:00:00Z',
      agent_level: 'core',
      session_id: 'session-valid',
      call_id: 'call-3',
      tool_name: 'bash',
      result: 'ok',
      duration: 20,
      tool_sla: {
        tool_name: 'bash',
        p50_latency_ms: 10,
        p95_latency_ms: 20,
        p99_latency_ms: 30,
        error_rate: 0,
        call_count: 1,
        success_rate: 1,
        cost_usd_total: 0.1,
        cost_usd_avg: 0.1,
      },
    };

    const result = normalizeAgentEvent(raw);

    expect(result.status).toBe('valid');
    expect(result.event?.tool_sla?.tool_name).toBe('bash');
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
