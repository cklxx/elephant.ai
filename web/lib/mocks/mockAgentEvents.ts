import { AgentEvent, AnyAgentEvent, AttachmentPayload } from '@/lib/types';

const HTML_ARTIFACT_PREVIEW =
  'data:text/html;base64,PCFkb2N0eXBlIGh0bWw+PGh0bWw+PGJvZHkgc3R5bGU9ImZvbnQtZmFtaWx5OkludGVyLHNhbnMtc2VyaWY7cGFkZGluZzoyNHB4O2JhY2tncm91bmQ6I2Y4ZmFmYyI+PGgxPkNvbnNvbGUgQXJjaGl0ZWN0dXJlIFByb3RvdHlwZTwvaDE+PHA+VGhpcyBhcnRpZmFjdCBkZW1vbnN0cmF0ZXMgdGhlIGlubGluZSBIVE1MIHByZXZpZXcgY2hhbm5lbC48L3A+PHVsPjxsaT5Eb2NrZWQgaW5wdXQ8L2xpPjxsaT5UaW1lbGluZSBzcGxpdCB2aWV3PC9saT48bGk+QXJ0aWZhY3QgZ2FsbGVyeTwvbGk+PC91bD48L2JvZHk+PC9odG1sPg==';

const MARKDOWN_ARTIFACT_PREVIEW =
  'data:text/html;base64,PCFkb2N0eXBlIGh0bWw+PGh0bWw+PGJvZHkgc3R5bGU9ImZvbnQtZmFtaWx5OkludGVyLHNhbnMtc2VyaWY7cGFkZGluZzoyNHB4O2JhY2tncm91bmQ6I2ZmZmJlNiI+PGgxPlEzIFJlc2VhcmNoIE1lbW88L2gxPjxwPkNvbnZlcnRlZCBmcm9tIG1hcmtkb3duIHRvIEhUTUwgZm9yIHByZXZpZXcgdGVzdGluZy48L3A+PGJsb2NrcXVvdGU+QXJ0aWZhY3QgcHJldmlld3Mgc2hvdWxkIGZlZWwgaW5zdGFudC48L2Jsb2NrcXVvdGU+PHA+LSBHb2FsczogdW5ibG9jayBVSSBkZW1vczwvcD48cD4tIE93bmVyczogQ29uc29sZSBzcXVhZDwvcD48L2JvZHk+PC9odG1sPg==';

const MARKDOWN_ARTIFACT_SOURCE =
  'data:text/markdown;base64,IyBRMyBSZXNlYXJjaCBNZW1vCgotIEdvYWxzOiBzaGFyZSBhcnRpZmFjdCBkZW1vCi0gT3duZXJzOiBDb25zb2xlIHNxdWFkCg==';

const ONBOARDING_MARKDOWN_SOURCE =
  'data:text/markdown;base64,IyMgT25ib2FyZGluZyBHdWlkZQoKLSBDbG9uZSB0aGUgcmVwbyBhbmQgaW5zdGFsbCBkZXBzLgotIFJ1biB0aGUgbW9jayBjb25zb2xlIHRvIHByZXZpZXcgYXJ0aWZhY3RzLgotIFZlcmlmeSBhdHRhY2htZW50cyByZW5kZXIgYmVmb3JlIHNoaXBwaW5nLgo=';

const CHECKLIST_MARKDOWN_SOURCE =
  'data:text/markdown;base64,IyBEZXByZWNhdGVkIENoZWNrbGlzdAoKLSBSZW1vdmUgc3RhbGUgc2NyZWVuc2hvdHMuCi0gQXJjaGl2ZSBvbGQgc3R1ZHkgZG9jcy4KLSBSZXBsYWNlIHdpdGggZnJlc2ggbWFya2Rvd24gcHJldmlld3MuCg==';

function createSvgDataUrl(color: string, label: string) {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 320 200"><rect width="320" height="200" rx="24" fill="${color}"/><text x="50%" y="50%" dominant-baseline="middle" text-anchor="middle" font-family="Inter, sans-serif" font-size="32" fill="#ffffff">${label}</text></svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

const mockAttachmentGallery: Record<string, AttachmentPayload> = {
  'Executive Review Slides': {
    name: 'Executive Review Slides',
    description: 'Executive Review Slides',
    media_type:
      'application/vnd.openxmlformats-officedocument.presentationml.presentation',
    format: 'pptx',
    kind: 'artifact',
    uri: 'https://mock.cdn.example.com/artifacts/executive-review-slides.pptx',
    preview_profile: 'document.presentation',
    preview_assets: [
      {
        asset_id: 'ppt-slide-1',
        label: 'Slide 1',
        mime_type: 'image/svg+xml',
        preview_type: 'image',
        cdn_url: createSvgDataUrl('#2563eb', 'Slide 1'),
      },
      {
        asset_id: 'ppt-slide-2',
        label: 'Slide 2',
        mime_type: 'image/svg+xml',
        preview_type: 'image',
        cdn_url: createSvgDataUrl('#7c3aed', 'Slide 2'),
      },
    ],
  },
  'Console Architecture Prototype': {
    name: 'Console Architecture Prototype',
    description: 'Console Architecture Prototype',
    media_type: 'text/html',
    format: 'html',
    kind: 'artifact',
    uri: HTML_ARTIFACT_PREVIEW,
    preview_profile: 'embed.html',
    preview_assets: [
      {
        asset_id: 'html-preview',
        label: 'Live preview',
        mime_type: 'text/html',
        preview_type: 'iframe',
        cdn_url: HTML_ARTIFACT_PREVIEW,
      },
    ],
  },
  'Q3 Research Memo': {
    name: 'Q3 Research Memo',
    description: 'Q3 Research Memo',
    media_type: 'text/markdown',
    format: 'markdown',
    kind: 'artifact',
    uri: MARKDOWN_ARTIFACT_SOURCE,
    preview_profile: 'document.markdown',
    preview_assets: [
      {
        asset_id: 'markdown-preview',
        label: 'Rendered memo',
        mime_type: 'text/html',
        preview_type: 'iframe',
        cdn_url: MARKDOWN_ARTIFACT_PREVIEW,
      },
    ],
  },
  'Onboarding Guide': {
    name: 'Onboarding Guide',
    description: 'Markdown onboarding guide with preview thumbnail',
    media_type: 'text/markdown',
    format: 'md',
    kind: 'artifact',
    uri: ONBOARDING_MARKDOWN_SOURCE,
    preview_profile: 'document.markdown',
    preview_assets: [
      {
        asset_id: 'guide-thumb',
        label: 'Guide thumbnail',
        mime_type: 'image/svg+xml',
        preview_type: 'image',
        cdn_url: createSvgDataUrl('#22c55e', 'Guide'),
      },
      {
        asset_id: 'guide-rendered',
        label: 'Rendered guide',
        mime_type: 'text/html',
        preview_type: 'iframe',
        cdn_url: MARKDOWN_ARTIFACT_PREVIEW,
      },
    ],
  },
  'Deprecated Checklist': {
    name: 'Deprecated Checklist',
    description: 'Cleanup list slated for removal',
    media_type: 'text/markdown',
    format: 'md',
    kind: 'artifact',
    uri: CHECKLIST_MARKDOWN_SOURCE,
    preview_profile: 'document.markdown',
    preview_assets: [
      {
        asset_id: 'checklist-thumb',
        label: 'Archive thumb',
        mime_type: 'image/svg+xml',
        preview_type: 'image',
        cdn_url: createSvgDataUrl('#f59e0b', 'Archive'),
      },
      {
        asset_id: 'checklist-render',
        label: 'Checklist details',
        mime_type: 'text/html',
        preview_type: 'iframe',
        cdn_url: MARKDOWN_ARTIFACT_PREVIEW,
      },
    ],
  },
  'Status Heatmap': {
    name: 'Status Heatmap',
    description: 'Status Heatmap',
    media_type: 'image/svg+xml',
    format: 'png',
    kind: 'attachment',
    uri: createSvgDataUrl('#f97316', 'Status'),
  },
  'Latency Report': {
    name: 'Latency Report',
    description: 'Latency Report',
    media_type: 'application/pdf',
    format: 'pdf',
    kind: 'artifact',
    uri: 'https://mock.cdn.example.com/artifacts/latency-report.pdf',
    preview_profile: 'document.pdf',
    preview_assets: [
      {
        asset_id: 'latency-preview',
        label: 'PDF preview',
        mime_type: 'image/svg+xml',
        preview_type: 'image',
        cdn_url: createSvgDataUrl('#0f172a', 'Latency'),
      },
    ],
  },
};

function cloneAttachmentMap(
  map: Record<string, AttachmentPayload>,
): Record<string, AttachmentPayload> {
  return Object.entries(map).reduce<Record<string, AttachmentPayload>>(
    (acc, [key, value]) => {
      acc[key] = {
        ...value,
        preview_assets: value.preview_assets
          ? value.preview_assets.map((asset) => ({ ...asset }))
          : undefined,
      };
      return acc;
    },
    {},
  );
}

function pickAttachments(...keys: string[]): Record<string, AttachmentPayload> {
  const selection: Record<string, AttachmentPayload> = {};
  keys.forEach((key) => {
    if (mockAttachmentGallery[key]) {
      selection[key] = mockAttachmentGallery[key];
    }
  });
  return cloneAttachmentMap(selection);
}

export type MockEventPayload = Partial<AnyAgentEvent> &
  Pick<AgentEvent, 'event_type' | 'agent_level'> &
  Partial<
    Pick<
      AgentEvent,
      'is_subtask' | 'parent_task_id' | 'subtask_index' | 'total_subtasks' | 'subtask_preview' | 'max_parallel'
    >
  > &
  Record<string, unknown>;

export interface TimedMockEvent {
  delay: number;
  event: MockEventPayload;
}

export function createMockEventSequence(_task: string): TimedMockEvent[] {
  const callId = 'mock-call-1';
  const parentTaskId = 'mock-core-task';

  const subtaskOneMeta = {
    is_subtask: true,
    parent_task_id: parentTaskId,
    subtask_index: 0,
    total_subtasks: 2,
    subtask_preview: 'Research comparable console UX patterns',
    max_parallel: 2,
  } as const;

  const subtaskTwoMeta = {
    is_subtask: true,
    parent_task_id: parentTaskId,
    subtask_index: 1,
    total_subtasks: 2,
    subtask_preview: 'Inspect tool output rendering implementation',
    max_parallel: 2,
  } as const;

  return [
    {
      delay: 950,
      event: {
        event_type: 'workflow.node.started',
        agent_level: 'core',
        step_index: 0,
        step_description: 'Collecting repository context',
      },
    },
    {
      delay: 1200,
      event: {
        event_type: 'workflow.node.started',
        agent_level: 'core',
        iteration: 1,
        total_iters: 3,
      },
    },
    {
      delay: 1450,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'core',
        iteration: 1,
        message_count: 1,
      },
    },
    {
      delay: 1700,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        iteration: 1,
        call_id: callId,
        tool_name: 'file_read',
        arguments: {
          path: 'web/app/page.tsx',
        },
      },
    },
    {
      delay: 1950,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'core',
        call_id: callId,
        chunk: 'Inspecting layout composition...\n',
        is_complete: false,
      },
    },
    {
      delay: 2150,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'core',
        call_id: callId,
        chunk: 'Detected conditional input rendering issue.\n',
        is_complete: false,
      },
    },
    {
      delay: 2350,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'core',
        call_id: callId,
        chunk: 'Preparing remediation suggestions...\n',
        is_complete: true,
      },
    },
    {
      delay: 2550,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: callId,
        tool_name: 'file_read',
        result:
          'Uploaded [Executive Review Slides], [Console Architecture Prototype], [Q3 Research Memo], [Status Heatmap], and [Latency Report] into the registry for downstream previews.',
        duration: 420,
        attachments: pickAttachments(
          'Executive Review Slides',
          'Console Architecture Prototype',
          'Q3 Research Memo',
          'Status Heatmap',
          'Latency Report',
        ),
      },
    },
    {
      delay: 2625,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        iteration: 1,
        call_id: 'mock-artifact-write',
        tool_name: 'artifacts_write',
        arguments: {
          path: 'attachments/onboarding-guide.md',
          format: 'markdown',
          operation: 'create',
        },
      },
    },
    {
      delay: 2850,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: 'mock-artifact-write',
        tool_name: 'artifacts_write',
        result:
          'Persisted markdown notes as [Onboarding Guide] and staged cleanup for [Deprecated Checklist].',
        duration: 360,
        attachments: pickAttachments('Onboarding Guide', 'Deprecated Checklist'),
        metadata: {
          attachment_mutations: {
            add: pickAttachments('Onboarding Guide'),
            update: pickAttachments('Deprecated Checklist'),
          },
        },
      },
    },
    {
      delay: 2950,
      event: {
        event_type: 'workflow.node.completed',
        agent_level: 'core',
        step_index: 0,
        step_result: 'Collected baseline UI findings',
      },
    },
    {
      delay: 2975,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        iteration: 1,
        call_id: 'mock-artifact-list',
        tool_name: 'artifacts_list',
        arguments: {
          kind: 'artifact',
          include_thumbnails: true,
        },
      },
    },
    {
      delay: 3150,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: 'mock-artifact-list',
        tool_name: 'artifacts_list',
        result:
          'Synced attachment catalog with thumbnails for downstream document views.',
        duration: 310,
        attachments: pickAttachments(
          'Executive Review Slides',
          'Console Architecture Prototype',
          'Q3 Research Memo',
          'Status Heatmap',
          'Latency Report',
          'Onboarding Guide',
        ),
        metadata: {
          attachment_mutations: {
            replace: pickAttachments(
              'Executive Review Slides',
              'Console Architecture Prototype',
              'Q3 Research Memo',
              'Status Heatmap',
              'Latency Report',
              'Onboarding Guide',
            ),
          },
        },
      },
    },
    {
      delay: 3200,
      event: {
        event_type: 'workflow.node.completed',
        agent_level: 'core',
        iteration: 1,
        tokens_used: 865,
        tools_run: 1,
      },
    },
    {
      delay: 3225,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        iteration: 1,
        call_id: 'mock-artifact-delete',
        tool_name: 'artifacts_delete',
        arguments: {
          names: ['Deprecated Checklist'],
          reason: 'remove temporary scratchpad',
        },
      },
    },
    {
      delay: 3285,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: 'mock-artifact-delete',
        tool_name: 'artifacts_delete',
        result:
          'Archived scratch attachment [Deprecated Checklist] to keep the gallery focused on final assets.',
        duration: 240,
        metadata: {
          attachment_mutations: {
            remove: ['Deprecated Checklist'],
          },
        },
      },
    },
    {
      delay: 3300,
      event: {
        event_type: 'workflow.node.started',
        agent_level: 'subagent',
        iteration: 1,
        total_iters: 1,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3350,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'subagent',
        iteration: 1,
        message_count: 1,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3400,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'subagent',
        iteration: 1,
        call_id: 'mock-subagent-call-1',
        tool_name: 'web_search',
        arguments: {
          query: 'subagent console ux parallel timeline inspiration',
        },
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3500,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-1',
        chunk: 'Summarizing multi-panel timelines from recent product launches...\n',
        is_complete: false,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3600,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-1',
        chunk: 'Highlighted research from Cursor, Windsurf, and Devina.\n',
        is_complete: true,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3750,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-1',
        tool_name: 'web_search',
        result:
          'Compiled documentation links covering responsive layouts and console UX patterns used in modern AI assistants.',
        duration: 780,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3950,
      event: {
        event_type: 'workflow.result.final',
        agent_level: 'subagent',
        final_answer:
          'Validated layout guidance from industry references and highlighted critical interaction affordances to emulate.',
        total_iterations: 1,
        total_tokens: 256,
        stop_reason: 'completed',
        duration: 900,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 4050,
      event: {
        event_type: 'workflow.node.started',
        agent_level: 'subagent',
        iteration: 1,
        total_iters: 1,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4100,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'subagent',
        iteration: 1,
        message_count: 1,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4150,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'subagent',
        iteration: 1,
        call_id: 'mock-subagent-call-2',
        tool_name: 'code_search',
        arguments: {
          path: 'web/components/agent/EventLine',
          query: 'subagent',
        },
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4325,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-2',
        chunk: 'Traced ToolOutputCard props to ensure subtask metadata is surfaced...\n',
        is_complete: false,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4480,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-2',
        chunk: 'Confirmed CSS tokens apply to subagent badges and dividers.\n',
        is_complete: true,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4650,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-2',
        tool_name: 'code_search',
        result:
          'Identified relevant components handling tool output rendering and confirmed subagent styling tokens are applied.',
        duration: 640,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4850,
      event: {
        event_type: 'workflow.result.final',
        agent_level: 'subagent',
        final_answer:
          'Confirmed ToolOutputCard handles metadata for subagent streams and recommended expanding automated coverage.',
        total_iterations: 1,
        total_tokens: 198,
        stop_reason: 'completed',
        duration: 880,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4250,
      event: {
        event_type: 'workflow.node.output.summary',
        agent_level: 'core',
        iteration: 1,
        content: 'Ready to summarize the refactor recommendations.',
        tool_call_count: 1,
      },
    },
    {
      delay: 4325,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'core',
        iteration: 1,
        delta: 'Here are the key findings from the console audit:\n',
        final: false,
      },
    },
    {
      delay: 4400,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'core',
        iteration: 1,
        delta: '- Keep the input dock always visible so tasks are effortless.\n',
        final: false,
      },
    },
    {
      delay: 4480,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'core',
        iteration: 1,
        delta:
          '- Stream agent output in a dedicated column for clarity.\n- Provide reconnection affordances with the new console styling.',
        final: true,
      },
    },
    {
      delay: 4510,
      event: {
        event_type: 'workflow.result.final',
        agent_level: 'core',
        final_answer:
          'Drafting summary...\n- Slides incoming: [Executive Review Slides]\n- HTML preview: [Console Architecture Prototype]',
        total_iterations: 1,
        total_tokens: 865,
        stop_reason: 'completed',
        duration: 3400,
      },
    },
    {
      delay: 4550,
      event: {
        event_type: 'workflow.result.final',
        agent_level: 'core',
        final_answer:
          '### Artifact delivery\n- Slides: [Executive Review Slides]\n- HTML preview: [Console Architecture Prototype]\n- Markdown memo: [Q3 Research Memo]\n- Team onboarding: [Onboarding Guide]\n- Visual context: [Status Heatmap]\n- PDF summary: [Latency Report]',
        total_iterations: 1,
        total_tokens: 865,
        stop_reason: 'completed',
        duration: 3600,
        attachments: pickAttachments(
          'Executive Review Slides',
          'Console Architecture Prototype',
          'Q3 Research Memo',
          'Onboarding Guide',
          'Status Heatmap',
          'Latency Report',
        ),
      },
    },
  ];
}
