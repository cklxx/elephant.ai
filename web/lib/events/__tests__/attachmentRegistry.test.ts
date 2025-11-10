import { describe, it, expect, beforeEach } from 'vitest';
import { handleAttachmentEvent, resetAttachmentRegistry } from '@/lib/events/attachmentRegistry';
import { TaskCompleteEvent, ToolCallCompleteEvent } from '@/lib/types';

const baseToolCallEvent = (): ToolCallCompleteEvent => ({
  event_type: 'tool_call_complete',
  agent_level: 'core',
  timestamp: new Date().toISOString(),
  session_id: 'sess-1',
  call_id: 'call-1',
  tool_name: 'seedream',
  result: 'ok',
  duration: 1200,
});

const baseTaskCompleteEvent = (): TaskCompleteEvent => ({
  event_type: 'task_complete',
  agent_level: 'core',
  timestamp: new Date().toISOString(),
  session_id: 'sess-1',
  final_answer: 'Done',
  total_iterations: 2,
  total_tokens: 256,
  stop_reason: 'final_answer',
  duration: 5000,
});

describe('attachmentRegistry', () => {
  beforeEach(() => {
    resetAttachmentRegistry();
  });

  it('hydrates task_complete events using previously captured attachments', () => {
    const toolEvent: ToolCallCompleteEvent = {
      ...baseToolCallEvent(),
      attachments: {
        'generated.png': {
          name: 'generated.png',
          media_type: 'image/png',
          data: 'YmFzZTY0',
        },
      },
    };
    handleAttachmentEvent(toolEvent);

    const taskComplete: TaskCompleteEvent = {
      ...baseTaskCompleteEvent(),
      final_answer: 'Artifacts ready: [generated.png]',
    };
    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeDefined();
    expect(taskComplete.attachments?.['generated.png']).toMatchObject({
      media_type: 'image/png',
      data: 'YmFzZTY0',
    });
  });

  it('does not leak attachments after reset', () => {
    const toolEvent: ToolCallCompleteEvent = {
      ...baseToolCallEvent(),
      attachments: {
        'temporary.png': {
          name: 'temporary.png',
          media_type: 'image/png',
          data: 'dGVtcA==',
        },
      },
    };
    handleAttachmentEvent(toolEvent);
    resetAttachmentRegistry();

    const taskComplete: TaskCompleteEvent = {
      ...baseTaskCompleteEvent(),
      final_answer: 'Check this out: [temporary.png]',
    };
    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeUndefined();
  });
});
