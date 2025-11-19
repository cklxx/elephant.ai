import { describe, expect, it } from 'vitest';
import { buildEventSignature } from '../useSSE';
import { AssistantMessageEvent } from '@/lib/types';

const baseAssistantEvent: AssistantMessageEvent = {
  event_type: 'assistant_message',
  agent_level: 'core',
  session_id: 'session-123',
  task_id: 'task-abc',
  iteration: 1,
  delta: 'Hello world',
  final: false,
  timestamp: '2025-01-01T00:00:00Z',
  created_at: '2025-01-01T00:00:00.000000001Z',
};

describe('buildEventSignature', () => {
  it('includes created_at so duplicate deltas within the same second remain unique', () => {
    const first = baseAssistantEvent;
    const second: AssistantMessageEvent = {
      ...baseAssistantEvent,
      created_at: '2025-01-01T00:00:00.000000002Z',
    };

    expect(buildEventSignature(first)).not.toEqual(
      buildEventSignature(second),
    );
  });
});
