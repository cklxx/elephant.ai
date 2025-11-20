import { describe, expect, it } from 'vitest';
import { SubagentEventDeriver } from '../subagentDeriver';
import { AnyAgentEvent } from '@/lib/types';

describe('SubagentEventDeriver', () => {
  it('derives progress and completion events from subtask streams', () => {
    const deriver = new SubagentEventDeriver();
    const baseEvent: AnyAgentEvent = {
      event_type: 'tool_call_complete',
      agent_level: 'subagent',
      session_id: 'session-1',
      task_id: 'task-sub-1',
      parent_task_id: 'task-root',
      timestamp: new Date().toISOString(),
      call_id: 'call-1',
      tool_name: 'bash',
      result: 'ok',
      duration: 1,
      is_subtask: true,
      subtask_index: 0,
      total_subtasks: 2,
    } as AnyAgentEvent;

    const firstDerived = deriver.derive(baseEvent);
    expect(firstDerived).toHaveLength(1);
    expect(firstDerived[0]).toMatchObject({
      event_type: 'subagent_progress',
      completed: 0,
      total: 2,
      tool_calls: 1,
      parent_task_id: 'task-root',
      task_id: 'task-root',
    });

    const completionEvent: AnyAgentEvent = {
      ...baseEvent,
      event_type: 'task_complete',
      total_tokens: 10,
    } as AnyAgentEvent;

    const secondDerived = deriver.derive(completionEvent);
    expect(secondDerived).toHaveLength(1);
    expect(secondDerived[0]).toMatchObject({
      event_type: 'subagent_progress',
      completed: 1,
      total: 2,
      tokens: 10,
    });

    const finalCompletion: AnyAgentEvent = {
      ...baseEvent,
      event_type: 'task_complete',
      subtask_index: 1,
      total_tokens: 20,
    } as AnyAgentEvent;

    const finalDerived = deriver.derive(finalCompletion);
    expect(finalDerived).toHaveLength(2);
    expect(finalDerived[0]).toMatchObject({
      event_type: 'subagent_progress',
      completed: 2,
      total: 2,
      tokens: 30,
      tool_calls: 1,
    });
    expect(finalDerived[1]).toMatchObject({
      event_type: 'subagent_complete',
      total: 2,
      success: 2,
      failed: 0,
      tokens: 30,
      tool_calls: 1,
    });
  });

  it('handles delegated events identified by agent_level even without is_subtask flag', () => {
    const deriver = new SubagentEventDeriver();
    const event: AnyAgentEvent = {
      event_type: 'tool_call_complete',
      agent_level: 'subagent',
      session_id: 'session-1',
      task_id: 'task-sub-2',
      parent_task_id: 'task-root',
      timestamp: new Date().toISOString(),
      call_id: 'call-2',
      tool_name: 'bash',
      result: 'ok',
      duration: 1,
      total_subtasks: 2,
    } as AnyAgentEvent;

    const derived = deriver.derive(event);
    expect(derived[0]).toMatchObject({
      event_type: 'subagent_progress',
      total: 2,
      tool_calls: 1,
      parent_task_id: 'task-root',
    });
  });

  it('uses subagent call prefix to classify delegated tool streams', () => {
    const deriver = new SubagentEventDeriver();
    const event: AnyAgentEvent = {
      event_type: 'task_complete',
      agent_level: 'core',
      session_id: 'session-1',
      task_id: 'task-root-sub',
      parent_task_id: 'task-root',
      timestamp: new Date().toISOString(),
      call_id: 'subagent:4',
      total_subtasks: 1,
      total_tokens: 3,
    } as AnyAgentEvent;

    const derived = deriver.derive(event);
    expect(derived).toHaveLength(2);
    expect(derived[0]).toMatchObject({
      event_type: 'subagent_progress',
      completed: 1,
      total: 1,
      tokens: 3,
      tool_calls: 0,
    });
    expect(derived[1]).toMatchObject({ event_type: 'subagent_complete', total: 1, success: 1 });
  });

  it('falls back to task_id when parent_task_id is missing on delegated streams', () => {
    const deriver = new SubagentEventDeriver();
    const event: AnyAgentEvent = {
      event_type: 'tool_call_complete',
      agent_level: 'subagent',
      session_id: 'session-1',
      task_id: 'task-sub-3',
      timestamp: new Date().toISOString(),
      call_id: 'subagent:7',
      tool_name: 'bash',
      result: 'ok',
      duration: 1,
      subtask_index: 0,
    } as AnyAgentEvent;

    const derived = deriver.derive(event);
    expect(derived[0]).toMatchObject({
      event_type: 'subagent_progress',
      completed: 0,
      total: 1,
      parent_task_id: 'task-sub-3',
      task_id: 'task-sub-3',
    });
  });

  it('infers totals from completion ordering when total_subtasks is omitted', () => {
    const deriver = new SubagentEventDeriver();
    const baseEvent: AnyAgentEvent = {
      event_type: 'task_complete',
      agent_level: 'subagent',
      session_id: 'session-1',
      task_id: 'task-root',
      parent_task_id: 'task-root',
      timestamp: new Date().toISOString(),
      total_tokens: 5,
    } as AnyAgentEvent;

    // First completion arrives for the final subtask index, which reveals total count
    const derivedFirst = deriver.derive({ ...baseEvent, subtask_index: 2 });
    expect(derivedFirst).toHaveLength(1);
    expect(derivedFirst[0]).toMatchObject({ event_type: 'subagent_progress', total: 3, completed: 1 });

    // Earlier completions arrive later; totals should already be locked to the inferred value
    deriver.derive({ ...baseEvent, subtask_index: 0, total_tokens: 3 });
    const derivedFinal = deriver.derive({ ...baseEvent, subtask_index: 1, total_tokens: 4 });

    expect(derivedFinal).toHaveLength(2);
    expect(derivedFinal[0]).toMatchObject({
      event_type: 'subagent_progress',
      completed: 3,
      total: 3,
      tokens: 12,
    });
    expect(derivedFinal[1]).toMatchObject({ event_type: 'subagent_complete', total: 3, success: 3 });
  });
});
