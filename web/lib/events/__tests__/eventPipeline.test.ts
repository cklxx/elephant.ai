import { describe, it, expect, beforeEach } from 'vitest';

import { EventPipeline } from '../eventPipeline';
import { AgentEventBus } from '../eventBus';
import { EventRegistry } from '../eventRegistry';
import type { AnyAgentEvent } from '@/lib/types';

const DEDUPE_MAX_ENTRIES = 4000;
const BASE_TIMESTAMP = Date.parse('2024-05-01T00:00:00.000Z');

function buildFinalEnvelope(index: number) {
  return {
    event_type: 'workflow.result.final' as const,
    session_id: 's-evict',
    run_id: 't-evict',
    timestamp: new Date(BASE_TIMESTAMP + index * 1000).toISOString(),
    final_answer: `answer-${index}`,
    payload: {
      final_answer: `answer-${index}`,
    },
  };
}

describe('EventPipeline deduplication', () => {
  let bus: AgentEventBus;
  let registry: EventRegistry;
  let pipeline: EventPipeline;
  let received: AnyAgentEvent[];

  beforeEach(() => {
    bus = new AgentEventBus();
    registry = new EventRegistry();
    pipeline = new EventPipeline({ bus, registry });
    received = [];
    bus.subscribe((event) => received.push(event));
  });

  it('emits validated events as-is', () => {
    const envelope = {
      event_type: 'workflow.result.final',
      session_id: 's1',
      run_id: 't1',
      timestamp: '2024-01-01T00:00:00Z',
      final_answer: 'hello',
      payload: {
        final_answer: 'hello',
      },
    };

    pipeline.process(envelope);

    expect(received).toHaveLength(1);
    expect(received[0].event_type).toBe('workflow.result.final');
  });

  it('accepts null attachments on final events and still dedupes duplicates', () => {
    const envelope = {
      event_type: 'workflow.result.final',
      session_id: 's-null',
      run_id: 't-null',
      timestamp: '2024-01-01T00:00:00Z',
      final_answer: 'hello',
      attachments: null,
      payload: {
        final_answer: 'hello',
        attachments: null,
      },
    };

    pipeline.process(envelope);
    pipeline.process({ ...envelope });

    expect(received).toHaveLength(1);
    expect(received[0].attachments).toBeNull();
  });

  it('allows streaming updates with new content to flow through', () => {
    const first = {
      event_type: 'workflow.result.final',
      session_id: 's-stream',
      run_id: 't-stream',
      timestamp: '2024-01-01T00:00:00Z',
      final_answer: 'partial',
      is_streaming: true,
      stream_finished: false,
    };
    const second = {
      ...first,
      timestamp: '2024-01-01T00:00:01Z',
      final_answer: 'partial + next',
    };

    pipeline.process(first);
    pipeline.process(second);

    expect(received).toHaveLength(2);
    expect(received[0].final_answer).toBe('partial');
    expect(received[1].final_answer).toBe('partial + next');
  });

  it('emits terminal results after streaming even when the content is unchanged', () => {
    const stream = {
      event_type: 'workflow.result.final',
      session_id: 's2',
      run_id: 't2',
      timestamp: '2024-02-01T00:00:00Z',
      final_answer: 'done',
      is_streaming: true,
      stream_finished: false,
    };
    const final = {
      ...stream,
      timestamp: '2024-02-01T00:00:01Z',
      is_streaming: false,
      stream_finished: true,
    };

    pipeline.process(stream);
    pipeline.process(final);

    expect(received).toHaveLength(2);
    expect(received[1].stream_finished).toBe(true);
  });

  it('clears dedupe state on reset', () => {
    const envelope = {
      event_type: 'workflow.result.final',
      session_id: 's3',
      run_id: 't3',
      timestamp: '2024-03-01T00:00:00Z',
      final_answer: 'hello',
      payload: {
        final_answer: 'hello',
      },
    };
    const duplicate = { ...envelope, timestamp: '2024-03-01T00:00:00Z' };

    pipeline.process(envelope);
    pipeline.reset();
    pipeline.process(duplicate);

    expect(received).toHaveLength(2);
  });

  it('keeps distinct tool events with identical timestamps when names differ', () => {
    const timestamp = '2024-04-01T00:00:00Z';
    const basePayload = {
      event_type: 'workflow.tool.completed',
      session_id: 'session-tool',
      run_id: 'task-tool',
      timestamp,
      tool_name: '',
      call_id: '',
      result: '',
      duration: 1,
    } satisfies AnyAgentEvent;

    pipeline.process({ ...basePayload, tool_name: 'plan' });
    pipeline.process({ ...basePayload, tool_name: 'clarify' });

    expect(received).toHaveLength(2);
    expect(received.map((e) => (e as AnyAgentEvent & { tool_name?: string }).tool_name)).toEqual([
      'plan',
      'clarify',
    ]);
  });

  it('evicts the oldest signature once the dedupe window exceeds max entries', () => {
    for (let i = 0; i <= DEDUPE_MAX_ENTRIES; i += 1) {
      pipeline.process(buildFinalEnvelope(i));
    }

    pipeline.process(buildFinalEnvelope(0));
    pipeline.process(buildFinalEnvelope(DEDUPE_MAX_ENTRIES));

    expect(received).toHaveLength(DEDUPE_MAX_ENTRIES + 2);
    expect(received.at(-1)?.final_answer).toBe('answer-0');
  });

  it('keeps duplicate suppression correct after ring wrap-around', () => {
    const totalUniqueEvents = DEDUPE_MAX_ENTRIES * 2 + 5;
    for (let i = 0; i < totalUniqueEvents; i += 1) {
      pipeline.process(buildFinalEnvelope(i));
    }

    pipeline.process(buildFinalEnvelope(totalUniqueEvents - 1));
    pipeline.process(buildFinalEnvelope(DEDUPE_MAX_ENTRIES + 4));
    pipeline.process(buildFinalEnvelope(DEDUPE_MAX_ENTRIES + 4));

    expect(received).toHaveLength(totalUniqueEvents + 1);
    expect(received.at(-1)?.final_answer).toBe(`answer-${DEDUPE_MAX_ENTRIES + 4}`);
  });
});
