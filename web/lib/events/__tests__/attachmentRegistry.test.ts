import { describe, it, expect, beforeEach } from 'vitest';
import { handleAttachmentEvent, ingestRecalledAttachments, resetAttachmentRegistry } from '@/lib/events/attachmentRegistry';
import { WorkflowResultFinalEvent, WorkflowToolCompletedEvent, WorkflowInputReceivedEvent } from '@/lib/types';

const baseToolCallEvent = (): WorkflowToolCompletedEvent => ({
  event_type: 'workflow.tool.completed',
  agent_level: 'core',
  timestamp: new Date().toISOString(),
  session_id: 'sess-1',
  call_id: 'call-1',
  tool_name: 'seedream',
  result: 'ok',
  duration: 1200,
});

const baseWorkflowResultFinalEvent = (): WorkflowResultFinalEvent => ({
  event_type: 'workflow.result.final',
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

  it('hydrates workflow.result.final events even when attachments were already shown', () => {
    const toolEvent: WorkflowToolCompletedEvent = {
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

    const taskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      final_answer: 'Artifacts ready: [generated.png]',
    };
    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeDefined();
    expect(taskComplete.attachments?.['generated.png']).toBeDefined();
  });

  it('does not leak attachments after reset', () => {
    const toolEvent: WorkflowToolCompletedEvent = {
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

    const taskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      final_answer: 'Check this out: [temporary.png]',
    };
    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeUndefined();
  });

  it('retains workflow.result.final attachments that were not previously displayed', () => {
    const taskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
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

  it('hydrates workflow.result.final events from registry when assets were not displayed', () => {
    const userTask: WorkflowInputReceivedEvent = {
      event_type: 'workflow.input.received',
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

    const taskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      final_answer: 'See [analysis.png] for reference.',
    };
    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeDefined();
    expect(taskComplete.attachments?.['analysis.png']).toBeDefined();
  });

  it('hydrates workflow.tool.completed events using stored attachments when missing', () => {
    const userTask: WorkflowInputReceivedEvent = {
      event_type: 'workflow.input.received',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'sess-2',
      task_id: 'task-2',
      parent_task_id: undefined,
      task: 'Summarize video',
      attachments: {
        'clip.mp4': {
          name: 'clip.mp4',
          media_type: 'video/mp4',
          data: 'YmluYXJ5',
        },
      },
    };
    handleAttachmentEvent(userTask);

    const toolComplete: WorkflowToolCompletedEvent = {
      ...baseToolCallEvent(),
      result: 'Rendered preview: [clip.mp4]',
    };

    handleAttachmentEvent(toolComplete);

    expect(toolComplete.attachments).toBeDefined();
    expect(toolComplete.attachments?.['clip.mp4']).toBeDefined();
  });

  it('surfaces undisplayed attachments when final results omit them', () => {
    const userTask: WorkflowInputReceivedEvent = {
      event_type: 'workflow.input.received',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'sess-3',
      task_id: 'task-3',
      parent_task_id: undefined,
      task: 'Upload assets for later',
      attachments: {
        'undisplayed.txt': {
          name: 'undisplayed.txt',
          media_type: 'text/plain',
          data: 'bGF0ZXI=',
        },
      },
    };

    handleAttachmentEvent(userTask);

    const taskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      final_answer: 'Task finished successfully.',
    };

    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeDefined();
    expect(taskComplete.attachments?.['undisplayed.txt']).toBeDefined();
  });

  it('hydrates attachments from metadata mutations even when attachments are absent', () => {
    const toolEvent: WorkflowToolCompletedEvent = {
      ...baseToolCallEvent(),
      metadata: {
        attachment_mutations: JSON.stringify({
          add: {
            'report.md': {
              name: 'report.md',
              media_type: 'text/markdown',
              uri: 'https://example.com/report.md',
            },
          },
        }),
      },
      attachments: undefined,
    };

    handleAttachmentEvent(toolEvent);
    expect(toolEvent.attachments?.['report.md']).toBeDefined();

    const camelCaseEvent: WorkflowToolCompletedEvent = {
      ...baseToolCallEvent(),
      call_id: 'call-2',
      metadata: {
        attachmentMutations: {
          add: {
            'summary.pdf': {
              name: 'summary.pdf',
              media_type: 'application/pdf',
              uri: 'https://example.com/summary.pdf',
            },
          },
        },
      },
    };

    handleAttachmentEvent(camelCaseEvent);
    expect(camelCaseEvent.attachments?.['summary.pdf']).toBeDefined();

    const taskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      final_answer: 'See [report.md] and [summary.pdf] for details.',
    };
    handleAttachmentEvent(taskComplete);

    expect(taskComplete.attachments).toBeDefined();
    expect(taskComplete.attachments?.['report.md']).toBeDefined();
    expect(taskComplete.attachments?.['summary.pdf']).toBeDefined();
  });

  it('does not leak attachments from newer tasks when events are processed in reverse order', () => {
    const newerTaskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      task_id: 'task-2',
      timestamp: '2024-01-02T12:00:00.000Z',
      attachments: {
        'future.md': {
          name: 'future.md',
          media_type: 'text/markdown',
          uri: 'https://example.com/future.md',
        },
      },
    };

    const earlierTaskComplete: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      task_id: 'task-1',
      timestamp: '2024-01-01T12:00:00.000Z',
      final_answer: 'First task complete with no attachments.',
    };

    handleAttachmentEvent(newerTaskComplete);
    handleAttachmentEvent(earlierTaskComplete);

    expect(newerTaskComplete.attachments?.['future.md']).toBeDefined();
    expect(earlierTaskComplete.attachments).toBeUndefined();
  });

  it('keeps recalled attachments hidden unless referenced', () => {
    ingestRecalledAttachments({
      'history.pdf': {
        name: 'history.pdf',
        media_type: 'application/pdf',
        uri: 'https://example.com/history.pdf',
        visibility: 'recalled',
      },
    }, Date.parse('2024-01-01T00:00:00.000Z'));

    const summaryOnly: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      final_answer: 'Summary only.',
    };
    handleAttachmentEvent(summaryOnly);
    expect(summaryOnly.attachments).toBeUndefined();

    const referenced: WorkflowResultFinalEvent = {
      ...baseWorkflowResultFinalEvent(),
      final_answer: 'See [history.pdf] for details.',
    };
    handleAttachmentEvent(referenced);
    expect(referenced.attachments?.['history.pdf']).toBeDefined();
    expect(referenced.attachments?.['history.pdf']?.visibility).toBe('recalled');
  });
});
