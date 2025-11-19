'use client';

import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { AnyAgentEvent, AssistantMessageEvent } from '@/lib/types';

const baseTime = new Date('2025-10-12T08:00:00Z').getTime();

function atOffset(seconds: number) {
  return new Date(baseTime + seconds * 1000).toISOString();
}

const previewSessionId = 'preview-session';
const previewTaskId = 'preview-task';
const baseEventContext = { session_id: previewSessionId, task_id: previewTaskId } as const;

const subagentOneContext = {
  ...baseEventContext,
  task_id: 'preview-subtask-1',
  agent_level: 'subagent' as const,
  parent_task_id: previewTaskId,
  is_subtask: true,
  subtask_index: 0,
  total_subtasks: 2,
  subtask_preview: '沉浸式事件流体验对标调研',
  max_parallel: 2,
};

const subagentTwoContext = {
  ...baseEventContext,
  task_id: 'preview-subtask-2',
  agent_level: 'subagent' as const,
  parent_task_id: previewTaskId,
  is_subtask: true,
  subtask_index: 1,
  total_subtasks: 2,
  subtask_preview: '验证工具输出组件的子任务样式',
  max_parallel: 2,
};

const mockEvents: AnyAgentEvent[] = [
  {
    ...baseEventContext,
    event_type: 'user_task',
    timestamp: atOffset(0),
    agent_level: 'core',
    task: '调研自动化代理的实时回传方案，并输出总结报告。',
  },
  {
    ...baseEventContext,
    event_type: 'task_analysis',
    timestamp: atOffset(8),
    agent_level: 'core',
    action_name: '梳理现有遥测与告警体系',
    goal: '了解现有链路瓶颈，确认可复用的事件与指标。',
  },
  {
    ...baseEventContext,
    event_type: 'research_plan',
    timestamp: atOffset(12),
    agent_level: 'core',
    plan_steps: [
      '快速扫面业内方案与指标',
      '对比事件流 UI 的实时反馈模式',
      '整理最佳实践并建议落地步骤',
    ],
    estimated_iterations: 3,
    estimated_tools: ['web_search', 'browser', 'bash'],
    estimated_duration_minutes: 32,
  },
  {
    ...baseEventContext,
    event_type: 'iteration_start',
    timestamp: atOffset(18),
    agent_level: 'core',
    iteration: 1,
    total_iters: 3,
  },
  {
    ...baseEventContext,
    event_type: 'thinking',
    timestamp: atOffset(22),
    agent_level: 'core',
    iteration: 1,
    message_count: 1,
  },
  {
    ...baseEventContext,
    event_type: 'think_complete',
    timestamp: atOffset(30),
    agent_level: 'core',
    iteration: 1,
    content: '先检索业内终端流展示，确认指标与交互模式。',
    tool_call_count: 2,
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_start',
    timestamp: atOffset(35),
    agent_level: 'core',
    iteration: 1,
    call_id: 'call-1',
    tool_name: 'web_search',
    arguments: {
      query: 'agent operations timeline best practices auto scrolling terminal',
    },
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_stream',
    timestamp: atOffset(40),
    agent_level: 'core',
    call_id: 'call-1',
    chunk: '🔍 找到 12 篇关于自动滚动事件流和操作面板的案例...',
    is_complete: false,
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_complete',
    timestamp: atOffset(46),
    agent_level: 'core',
    call_id: 'call-1',
    tool_name: 'web_search',
    result: '聚合出 3 个实时流 UI 的滚动策略与指标采集模式。',
    duration: 4800,
  },
  {
    ...subagentOneContext,
    event_type: 'tool_call_start',
    timestamp: atOffset(48),
    iteration: 1,
    call_id: 'sub-call-1',
    tool_name: 'web_search',
    arguments: {
      query: 'multi-panel agent console layout inspiration',
    },
  },
  {
    ...subagentOneContext,
    event_type: 'tool_call_stream',
    timestamp: atOffset(50),
    call_id: 'sub-call-1',
    chunk: '📚 收集 GitHub Copilot 与 Cursor 控制台的排版策略...\n',
    is_complete: false,
  },
  {
    ...subagentOneContext,
    event_type: 'tool_call_stream',
    timestamp: atOffset(52),
    call_id: 'sub-call-1',
    chunk: '强调「工具列 + 时间线」分屏，加上高对比 badge。\n',
    is_complete: true,
  },
  {
    ...subagentOneContext,
    event_type: 'tool_call_complete',
    timestamp: atOffset(54),
    call_id: 'sub-call-1',
    tool_name: 'web_search',
    result: '归纳出 5 条关于多窗口事件回传的模式可供采用。',
    duration: 3200,
  },
  {
    ...baseEventContext,
    event_type: 'browser_info',
    timestamp: atOffset(50),
    agent_level: 'core',
    captured: new Date(Date.now() + 5000).toISOString(),
    success: true,
    message: 'Sandbox browser ready',
    user_agent: 'ConsolePreview/1.0',
    cdp_url: 'ws://console.example.com/devtools',
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_start',
    timestamp: atOffset(54),
    agent_level: 'core',
    iteration: 1,
    call_id: 'call-2',
    tool_name: 'bash',
    arguments: {
      command: 'npm run test -- research-timeline-autoscroll',
    },
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_stream',
    timestamp: atOffset(58),
    agent_level: 'core',
    call_id: 'call-2',
    chunk: '执行集成测试...\n> checking autoscroll state transitions',
    is_complete: false,
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_complete',
    timestamp: atOffset(63),
    agent_level: 'core',
    call_id: 'call-2',
    tool_name: 'bash',
    result: '',
    error: 'Test suite failed: autoscroll hook did not release focus',
    duration: 6200,
  },
  {
    ...subagentTwoContext,
    event_type: 'tool_call_start',
    timestamp: atOffset(64),
    iteration: 1,
    call_id: 'sub-call-2',
    tool_name: 'code_search',
    arguments: {
      path: 'web/components/agent/ToolOutputCard.tsx',
      query: 'subtask',
    },
  },
  {
    ...subagentTwoContext,
    event_type: 'tool_call_stream',
    timestamp: atOffset(66),
    call_id: 'sub-call-2',
    chunk: '比对 props 传递链路，确认 subtask metadata 是否完整...',
    is_complete: false,
  },
  {
    ...subagentTwoContext,
    event_type: 'tool_call_stream',
    timestamp: atOffset(69),
    call_id: 'sub-call-2',
    chunk: '需要在 mock 数据中加入 parent_task_id 与并行系数。',
    is_complete: true,
  },
  {
    ...subagentTwoContext,
    event_type: 'tool_call_complete',
    timestamp: atOffset(72),
    call_id: 'sub-call-2',
    tool_name: 'code_search',
    result: '确认 EventLine 组件渲染子任务标题，建议补测试覆盖。',
    duration: 3600,
  },
  {
    ...subagentOneContext,
    event_type: 'task_complete',
    timestamp: atOffset(73),
    final_answer: '完成对标调研，输出 badge 体系建议。',
    total_iterations: 1,
    total_tokens: 2400,
    stop_reason: 'completed',
    duration: 4200,
  },
  {
    ...subagentTwoContext,
    event_type: 'task_complete',
    timestamp: atOffset(74),
    final_answer: '补齐子任务工具事件 Mock，确保 UI 预览对齐。',
    total_iterations: 1,
    total_tokens: 2100,
    stop_reason: 'completed',
    duration: 4000,
  },
  {
    ...baseEventContext,
    event_type: 'iteration_complete',
    timestamp: atOffset(70),
    agent_level: 'core',
    iteration: 1,
    tokens_used: 8240,
    tools_run: 2,
  },
  {
    ...baseEventContext,
    event_type: 'assistant_message',
    timestamp: atOffset(74),
    created_at: atOffset(74),
    agent_level: 'core',
    iteration: 1,
    delta: '第一轮总结：搜索结果显示领先团队都实现了逐 token 更新。',
    final: false,
  },
  {
    ...baseEventContext,
    event_type: 'assistant_message',
    timestamp: atOffset(76),
    created_at: atOffset(76),
    agent_level: 'core',
    iteration: 1,
    delta: '我们需要在终端流中引入渐进式渲染来降低用户等待。',
    final: true,
  },
  {
    ...baseEventContext,
    event_type: 'iteration_start',
    timestamp: atOffset(78),
    agent_level: 'core',
    iteration: 2,
    total_iters: 3,
  },
  {
    ...baseEventContext,
    event_type: 'thinking',
    timestamp: atOffset(82),
    agent_level: 'core',
    iteration: 2,
    message_count: 1,
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_start',
    timestamp: atOffset(88),
    agent_level: 'core',
    iteration: 2,
    call_id: 'call-3',
    tool_name: 'browser',
    arguments: {
      url: 'https://design-system.example.com/terminal-stream',
      selector: '#live-timeline',
    },
  },
  {
    ...baseEventContext,
    event_type: 'tool_call_stream',
    timestamp: atOffset(94),
    agent_level: 'core',
    call_id: 'call-3',
    chunk: '📸 Captured DOM outline and streaming transcript snippet...',
    is_complete: false,
  },
  {
    ...baseEventContext,
    event_type: 'error',
    timestamp: atOffset(102),
    agent_level: 'core',
    iteration: 2,
    phase: 'execute',
    error: '等待浏览器快照响应超时，准备重试。',
    recoverable: true,
  },
  {
    ...baseEventContext,
    event_type: 'iteration_complete',
    timestamp: atOffset(112),
    agent_level: 'core',
    iteration: 2,
    tokens_used: 9620,
    tools_run: 1,
  },
  {
    ...baseEventContext,
    event_type: 'assistant_message',
    timestamp: atOffset(120),
    created_at: atOffset(120),
    agent_level: 'core',
    iteration: 2,
    delta: '第二轮调研补充了浏览器端的实时回传模式，建议结合。',
    final: false,
  },
  {
    ...baseEventContext,
    event_type: 'assistant_message',
    timestamp: atOffset(132),
    created_at: atOffset(132),
    agent_level: 'core',
    iteration: 2,
    delta: '最终方案：同时保留工具状态区与逐字增长的主回答气泡。',
    final: true,
  },
  {
    ...baseEventContext,
    event_type: 'task_complete',
    timestamp: atOffset(150),
    agent_level: 'core',
    final_answer:
      '整理出实时工具流的自动滚动策略，并给出逐步落地建议。',
    total_iterations: 2,
    total_tokens: 30871,
    stop_reason: 'end',
    duration: 13650,
  },
];

const summaryLine = buildSummaryLine(mockEvents);
const previewInput = buildPreviewInput(mockEvents);
const previewOutputs = buildPreviewOutputs(mockEvents);

export default function ConsolePreviewPage() {
  return (
    <div className="min-h-screen bg-slate-100 px-6 py-10">
      <div className="mx-auto flex max-w-4xl flex-col gap-8">
        <header className="space-y-2">
          <p className="text-[10px] font-semibold uppercase tracking-[0.4em] text-slate-400">
            Dev Preview · Mocked Data
          </p>
          <h1 className="text-2xl font-semibold text-slate-900">
            多轮工具调用事件流（Phase 4 无框排版示例）
          </h1>
          <p className="max-w-2xl text-sm text-slate-600">
            该页面通过静态数据模拟三轮工具调用：成功的搜索、失败的 Bash 测试以及仍在执行的浏览器采集，以验证所有时间线样式与状态标签在新排版下的表现。
          </p>
        </header>

        <section className="rounded-3xl bg-white/60 p-6 shadow-sm ring-1 ring-white/70">
          <TerminalOutput
            events={mockEvents}
            isConnected
            isReconnecting={false}
            error={null}
            reconnectAttempts={0}
            onReconnect={() => {}}
          />
        </section>

        <section className="rounded-3xl bg-white/80 p-6 shadow-sm ring-1 ring-white/70">
          <header className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
            <div className="space-y-1">
              <h2 className="text-lg font-semibold text-slate-900">
                对话快照
              </h2>
              <p className="text-xs text-slate-500">
                {summaryLine}
              </p>
            </div>
          </header>

          <div className="mt-6 grid gap-6 md:grid-cols-2">
            <article className="flex flex-col gap-4 rounded-2xl border border-slate-200/80 bg-slate-50/80 p-5">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.25em] text-slate-400">
                  Input
                </div>
                <p className="mt-2 text-sm leading-6 text-slate-700">
                  {previewInput.primary}
                </p>
              </div>

              {previewInput.supporting && (
                <div className="rounded-xl border border-slate-200 bg-white/80 p-4 text-xs leading-6 text-slate-600">
                  <p className="font-medium text-slate-500">研究计划</p>
                  <ul className="mt-2 list-disc space-y-1 pl-4 text-slate-600">
                    {previewInput.supporting.map((step, index) => (
                      <li key={index}>{step}</li>
                    ))}
                  </ul>
                </div>
              )}
            </article>

            <article className="flex flex-col gap-4 rounded-2xl border border-slate-200/80 bg-slate-900/90 p-5 text-slate-100">
              <div>
                <div className="text-[11px] font-semibold uppercase tracking-[0.25em] text-slate-300">
                  Output
                </div>
              </div>

              <div className="space-y-4">
                {previewOutputs.map((output) => (
                  <div
                    key={`${output.iteration}-${output.content}`}
                    className="rounded-xl border border-white/10 bg-white/5 p-4"
                  >
                    <div className="text-[11px] font-medium uppercase tracking-[0.2em] text-white/70">
                      Iteration {output.iteration}
                    </div>
                    <p className="mt-2 text-sm leading-6 text-slate-100">
                      {output.content}
                    </p>
                  </div>
                ))}
              </div>

              {previewInput.summary && (
                <div className="rounded-xl border border-amber-400/40 bg-amber-500/10 p-4 text-xs leading-6 text-amber-100">
                  <p className="font-medium text-amber-200">最终总结</p>
                  <p className="mt-2 text-slate-100">{previewInput.summary}</p>
                </div>
              )}
            </article>
          </div>
        </section>
      </div>
    </div>
  );
}

function findEvent<TEventType extends AnyAgentEvent['event_type']>(
  events: AnyAgentEvent[],
  eventType: TEventType,
): Extract<AnyAgentEvent, { event_type: TEventType }> | undefined {
  return events.find(
    (event): event is Extract<AnyAgentEvent, { event_type: TEventType }> =>
      event.event_type === eventType,
  );
}

function buildSummaryLine(events: AnyAgentEvent[]): string {
  const taskComplete = findEvent(events, 'task_complete');

  const iterations = taskComplete?.total_iterations;
  const tokens = taskComplete?.total_tokens;
  const durationSeconds = taskComplete?.duration
    ? (taskComplete.duration / 1000).toFixed(2)
    : undefined;

  const parts: string[] = [];
  if (iterations !== undefined) {
    parts.push(`${iterations} iterations`);
  }
  if (tokens !== undefined) {
    parts.push(`${tokens.toLocaleString('en-US')} tokens`);
  }
  if (durationSeconds !== undefined) {
    parts.push(`${durationSeconds}s`);
  }

  return parts.join(' · ');
}

function buildPreviewInput(events: AnyAgentEvent[]) {
  const userTask = findEvent(events, 'user_task');
  const planEvent = findEvent(events, 'research_plan');
  const taskComplete = findEvent(events, 'task_complete');

  return {
    primary:
      userTask?.task ?? '暂无输入，等待用户任务。',
    supporting: planEvent?.plan_steps ?? null,
    summary: taskComplete?.final_answer ?? null,
  };
}

type PreviewBucket = { key: string; iteration: number; content: string };

function buildPreviewOutputs(events: AnyAgentEvent[]): {
  iteration: number;
  content: string;
}[] {
  const buckets: PreviewBucket[] = [];
  const bucketMap = new Map<string, PreviewBucket>();

  events.forEach((event) => {
    if (event.event_type !== 'assistant_message') {
      return;
    }

    const assistantEvent = event as AssistantMessageEvent;
    const iteration = assistantEvent.iteration ?? 0;
    const key = `${assistantEvent.task_id ?? 'task'}:${assistantEvent.parent_task_id ?? 'root'}:${iteration}`;
    let bucket = bucketMap.get(key);
    if (!bucket) {
      bucket = { key, iteration, content: '' };
      bucketMap.set(key, bucket);
      buckets.push(bucket);
    }

    if (assistantEvent.delta) {
      bucket.content += assistantEvent.delta;
    }
  });

  return buckets
    .map((bucket) => ({
      iteration: bucket.iteration,
      content: bucket.content.trim(),
    }))
    .filter((bucket) => bucket.content.length > 0);
}
