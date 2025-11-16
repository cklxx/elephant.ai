import { describe, expect, it } from 'vitest';
import {
  createMockEventSequence,
  type MockEventPayload,
  type TimedMockEvent,
} from '../mockAgentEvents';

const FALLBACK_TASK = 'Analyze the repository and suggest improvements';

const isSubagentEvent = (event: MockEventPayload) => event.agent_level === 'subagent';

const asSubtaskMeta = (event: MockEventPayload) =>
  event as MockEventPayload & {
    is_subtask: boolean;
    parent_task_id: string;
    subtask_index: number;
    total_subtasks: number;
  };

describe('createMockEventSequence', () => {
  const sequence = createMockEventSequence('');

  it('falls back to default task description when no task provided', () => {
    const analysisEvent = sequence.find(({ event }) => event.event_type === 'task_analysis');

    expect(analysisEvent).toBeDefined();
    expect((analysisEvent?.event as MockEventPayload).goal).toBe(FALLBACK_TASK);
  });

  it('ensures subagent tool events include subtask metadata', () => {
    const subagentToolEvents = sequence.filter(
      ({ event }) =>
        isSubagentEvent(event) &&
        ['tool_call_start', 'tool_call_stream', 'tool_call_complete', 'task_complete'].includes(
          event.event_type,
        ),
    );

    expect(subagentToolEvents.length).toBeGreaterThan(0);

    for (const timedEvent of subagentToolEvents) {
      const meta = asSubtaskMeta(timedEvent.event);
      expect(meta.is_subtask).toBe(true);
      expect(meta.parent_task_id).toBe('mock-core-task');
      expect(meta.subtask_index).toBeGreaterThanOrEqual(0);
      expect(meta.total_subtasks).toBeGreaterThan(meta.subtask_index);
    }
  });

  it('adds preparatory iteration events for each subagent timeline', () => {
    const getBySubtask = (index: number) =>
      sequence.filter(({ event }) => isSubagentEvent(event) && event.subtask_index === index);

    const firstSubagentEvents = getBySubtask(0);
    const secondSubagentEvents = getBySubtask(1);

    const expectPrepEvents = (events: TimedMockEvent[]) => {
      expect(events.some(({ event }) => event.event_type === 'iteration_start')).toBe(true);
      expect(events.some(({ event }) => event.event_type === 'thinking')).toBe(true);
    };

    expectPrepEvents(firstSubagentEvents);
    expectPrepEvents(secondSubagentEvents);
  });
});
