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

  it('does not duplicate attachments already shown via tool events', () => {
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

    expect(taskComplete.attachments).toBeUndefined();
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

  it('retains task_complete attachments that were not previously displayed', () => {
    const taskComplete: TaskCompleteEvent = {
      ...baseTaskCompleteEvent(),
      attachments: {
        'fresh.png': {
          name: 'fresh.png',
          media_type: 'image/png',
          data: 'ZGF0YQ==',
        },
      },
    };

    handleAttachmentEvent(taskComplete);
    expect(taskComplete.attachments).toBeDefined();
    expect(taskComplete.attachments?.['fresh.png']).toBeDefined();
  });

  it('hydrates task_complete events from registry when assets were not displayed', () => {
    const userTask: UserTaskEvent = {
      event_type: 'user_task',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'sess-1',
      task_id: 'task-1',
      parent_task_id: undefined,
      task: 'Describe attachment',
      attachments: {
        'analysis.png': {
          name: 'analysis.png',
          media_type: 'image/png',
          data: 'YW5hbHlzaXM=',
        },
      },
    };
    handleAttachmentEvent(userTask);

    const taskComplete: TaskCompleteEvent = {
      ...baseTaskCompleteEvent(),
      final_answer: 'See [analysis.png] for reference.',
    };
    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeDefined();
    expect(taskComplete.attachments?.['analysis.png']).toBeDefined();
  });
});
