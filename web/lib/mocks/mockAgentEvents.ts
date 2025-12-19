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
  const runId = 'mock-run-1';
  const parentTaskId = 'mock-core-task';
  const taskIdOne = 'task-1';
  const taskIdTwo = 'task-2';
  const callIdPlan = 'mock-call-plan';
  const callIdClearifyOne = 'mock-call-clearify-1';
  const callIdClearifyTwo = 'mock-call-clearify-2';
  const callIdOne = 'mock-call-1';
  const callIdTwo = 'mock-call-2';
  const subagentCallIdOne = 'mock-subagent-call-1';
  const subagentCallIdTwo = 'mock-subagent-call-2';

  return [
    {
      delay: 350,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: callIdPlan,
        tool_name: 'plan',
        result: '优化控制台交互体验',
        duration: 120,
        metadata: {
          run_id: runId,
          overall_goal_ui: '优化控制台交互体验',
          complexity: 'complex',
          internal_plan: {
            overall_goal: '优化控制台交互体验',
            branches: [
              {
                branch_goal: '梳理现状',
                tasks: [{ task_goal: '收集现有 UI 事件流', success_criteria: ['定位旧展示路径'] }],
              },
              {
                branch_goal: '重构展示',
                tasks: [{ task_goal: '实现新的层级视图', success_criteria: ['替换旧 timeline 逻辑'] }],
              },
            ],
          },
        },
      },
    },
    {
      delay: 750,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: callIdClearifyOne,
        tool_name: 'clearify',
        result: '收集现有 UI 事件流',
        duration: 80,
        metadata: {
          run_id: runId,
          task_id: taskIdOne,
          task_goal_ui: '收集现有 UI 事件流',
          success_criteria: ['定位旧展示路径', '确认需要替换的组件与聚合逻辑'],
        },
      },
    },
    {
      delay: 980,
      event: {
        event_type: 'workflow.node.output.summary',
        agent_level: 'core',
        iteration: 1,
        content: '我先读取前端事件流与时间线相关文件，定位旧计划展示的位置。',
        tool_call_count: 1,
      },
    },
    {
      delay: 1200,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        iteration: 1,
        call_id: callIdOne,
        tool_name: 'file_read',
        arguments: {
          path: 'web/app/conversation/ConversationPageContent.tsx',
        },
      },
    },
    {
      delay: 1450,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: callIdOne,
        tool_name: 'file_read',
        result: 'Located legacy plan/timeline rendering path and right panel composition.',
        duration: 420,
      },
    },
    {
      delay: 1900,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: callIdClearifyTwo,
        tool_name: 'clearify',
        result: '实现新的层级视图',
        duration: 80,
        metadata: {
          run_id: runId,
          task_id: taskIdTwo,
          task_goal_ui: '实现新的层级视图',
          success_criteria: ['渲染 Goal/Task/Log 三层结构', '工具输出与日志保持一致字体'],
        },
      },
    },
    {
      delay: 2120,
      event: {
        event_type: 'workflow.node.output.summary',
        agent_level: 'core',
        iteration: 2,
        content: '准备渲染新的层级视图，并接入右侧工具详情面板。',
        tool_call_count: 1,
      },
    },
    {
      delay: 2350,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'core',
        iteration: 2,
        call_id: callIdTwo,
        tool_name: 'file_edit',
        arguments: {
          path: 'web/components/agent/PlannerReactView.tsx',
        },
      },
    },
    {
      delay: 2680,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'core',
        call_id: callIdTwo,
        tool_name: 'file_edit',
        result: 'Added Planner/ReAct view layout and task/action mapping.',
        duration: 560,
        attachments: pickAttachments('Console Architecture Prototype'),
      },
    },
    {
      delay: 2900,
      event: {
        event_type: 'workflow.node.output.summary',
        agent_level: 'core',
        iteration: 3,
        content: '我会收尾并给出最终总结。之后不再调用工具。',
        tool_call_count: 0,
      },
    },
    {
      delay: 3300,
      event: {
        event_type: 'workflow.result.final',
        agent_level: 'core',
        final_answer: '已完成 Planner/ReAct 架构展示与工具详情联动。',
        total_iterations: 2,
        total_tokens: 800,
        stop_reason: 'final_answer',
        duration: 4200,
      },
    },
    // Subagent thread 1/2
    {
      delay: 3600,
      event: {
        event_type: 'workflow.node.started',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 0,
        total_subtasks: 2,
        subtask_preview: 'Subagent: gather docs',
        iteration: 1,
      },
    },
    {
      delay: 3700,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 0,
        total_subtasks: 2,
        delta: 'Thinking...',
        iteration: 1,
      },
    },
    {
      delay: 3800,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 0,
        total_subtasks: 2,
        call_id: subagentCallIdOne,
        tool_name: 'web_search',
        arguments: { query: 'plan/clearify protocol' },
        iteration: 1,
      },
    },
    {
      delay: 3880,
      event: {
        event_type: 'workflow.tool.progress',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 0,
        total_subtasks: 2,
        call_id: subagentCallIdOne,
        chunk: 'Searching...',
      },
    },
    {
      delay: 3980,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 0,
        total_subtasks: 2,
        call_id: subagentCallIdOne,
        tool_name: 'web_search',
        result: 'Found docs and examples.',
        duration: 180,
      },
    },
    {
      delay: 4100,
      event: {
        event_type: 'workflow.result.final',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 0,
        total_subtasks: 2,
        final_answer: 'Subagent summary: gathered protocol constraints and UI levels.',
        total_iterations: 1,
        total_tokens: 220,
        stop_reason: 'final_answer',
        duration: 900,
      },
    },
    // Subagent thread 2/2
    {
      delay: 4400,
      event: {
        event_type: 'workflow.node.started',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 1,
        total_subtasks: 2,
        subtask_preview: 'Subagent: verify UI',
        iteration: 1,
      },
    },
    {
      delay: 4480,
      event: {
        event_type: 'workflow.node.output.delta',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 1,
        total_subtasks: 2,
        delta: 'Reviewing UI...',
        iteration: 1,
      },
    },
    {
      delay: 4580,
      event: {
        event_type: 'workflow.tool.started',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 1,
        total_subtasks: 2,
        call_id: subagentCallIdTwo,
        tool_name: 'file_read',
        arguments: { path: 'web/components/agent/EventLine/index.tsx' },
        iteration: 1,
      },
    },
    {
      delay: 4680,
      event: {
        event_type: 'workflow.tool.completed',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 1,
        total_subtasks: 2,
        call_id: subagentCallIdTwo,
        tool_name: 'file_read',
        result: 'Reviewed EventLine implementation.',
        duration: 140,
      },
    },
    {
      delay: 4800,
      event: {
        event_type: 'workflow.result.final',
        agent_level: 'subagent',
        is_subtask: true,
        parent_task_id: parentTaskId,
        subtask_index: 1,
        total_subtasks: 2,
        final_answer: 'Subagent summary: UI renders Goal/Task/Log correctly; fonts consistent.',
        total_iterations: 1,
        total_tokens: 180,
        stop_reason: 'final_answer',
        duration: 850,
      },
    },
  ];
}
