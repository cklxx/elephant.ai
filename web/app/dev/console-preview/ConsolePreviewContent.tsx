'use client';

import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { AnyAgentEvent, WorkflowNodeOutputDeltaEvent } from '@/lib/types';
import { Brain, CheckCircle2, ChevronDown, Sparkles } from 'lucide-react';
import type { ReactNode } from 'react';

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

type SubagentStatus = 'pending' | 'running' | 'completed';
type ToolStatus = 'running' | 'completed' | 'blocked';
type StatusTone = 'muted' | 'info' | 'success' | 'warning';

interface SubagentToolEntry {
  id: string;
  label: string;
  summary: string;
  detail: string;
  duration: string;
  status: ToolStatus;
}

interface SubagentMission {
  id: string;
  title: string;
  preview: string;
  status: SubagentStatus;
  outcome: string;
  outputDeltas: string[];
  tools: SubagentToolEntry[];
}

interface ThinkMoment {
  id: string;
  content: string;
  accent: string;
}

interface DelegationMoment {
  id: string;
  title: string;
  detail: string;
  targetSubagentId: string;
  accent: string;
}

interface FinalToolPreview {
  title: string;
  description: string;
  expectedResult: string;
  highlights: { label: string; value: string }[];
}

const orchestrationSubagents: SubagentMission[] = [
  {
    id: 'immersive-ux',
    title: '沉浸式事件流对标',
    preview: '沉浸式事件流体验对标调研',
    status: 'completed',
    outcome: '整理 4 套竞品 UI，输出可直接复用的 badge 与分栏节奏。',
    outputDeltas: [
      '列出 Cursor、GitHub Copilot 等对标产品的控制台。',
      '抓取滚动节奏与实时状态标记。',
      '提炼哪些视觉 token 可以在当前排版沿用。',
    ],
    tools: [
      {
        id: 'ux-tool-1',
        label: '截屏竞品控制台',
        summary: '抓取 Cursor、Claude、Devin 的实时工具流排版。',
        detail: '生成 12 张对比图 + DOM 注释。',
        duration: '03:20',
        status: 'completed',
      },
      {
        id: 'ux-tool-2',
        label: '提炼 badge 体系',
        summary: '把竞品的状态颜色映射到 Spinner token。',
        detail: '输出 primary/muted/emphasis 三档对比表。',
        duration: '01:45',
        status: 'completed',
      },
    ],
  },
  {
    id: 'tooling-audit',
    title: '工具输出调试',
    preview: '验证工具输出组件的子任务样式',
    status: 'completed',
    outcome: '确认 ToolOutputCard 在 subagent 流中保持折叠/展开策略。',
    outputDeltas: [
      '确认 mock 事件里包含 parent_task_id、max_parallel。',
      '为工具输出补充 metadata, attachments 情况。',
      '设计子任务完成后的结果摘要。',
    ],
    tools: [
      {
        id: 'tooling-tool-1',
        label: '追踪 mock 事件',
        summary: '扫描 lib/mocks/mockAgentEvents.ts 的字段覆盖率。',
        detail: '新增 call_id + subtask_preview 的断言。',
        duration: '02:10',
        status: 'completed',
      },
      {
        id: 'tooling-tool-2',
        label: '渲染子任务卡片',
        summary: '验证 SubagentHeader + ToolOutputCard 组合。',
        detail: 'Storybook 中截图 3 种状态。',
        duration: '01:05',
        status: 'completed',
      },
    ],
  },
  {
    id: 'replay-script',
    title: '回放脚本',
    preview: '录制自动化脚本，回放 Subagent 时间线',
    status: 'running',
    outcome: '构建浏览器脚本，确保事件流在回放模式下同步滚动。',
    outputDeltas: [
      '拆分录制脚本与可视化组件的耦合。',
      '确定滚动锚点与“跳转最新”行为。',
    ],
    tools: [
      {
        id: 'replay-tool-1',
        label: '录制滚动轨迹',
        summary: '在回放模式内记录事件元素的位置。',
        detail: '产出 6 条滚动锚点 + 节奏曲线。',
        duration: '进行中',
        status: 'running',
      },
      {
        id: 'replay-tool-2',
        label: '生成回放脚本',
        summary: '把锚点转成 playwright 指令，方便自动演示。',
        detail: '等待滚动数据完成。',
        duration: '排队',
        status: 'blocked',
      },
    ],
  },
];

const mainAgentThinkMoments: ThinkMoment[] = [
  {
    id: 'think-1',
    content: '需要拆成“体验调研 + 组件验证 + 回放脚本”三路并行。',
    accent: 'Iteration 01 · 00:22',
  },
  {
    id: 'think-2',
    content: '持续同步每个 subagent 的回传，避免工具卡片重复。',
    accent: 'Iteration 01 · 00:48',
  },
  {
    id: 'think-3',
    content: '在最终总结中合并 badge token 与滚动策略。',
    accent: 'Iteration 02 · 01:12',
  },
];

const mainAgentDelegations: DelegationMoment[] = [
  {
    id: 'delegate-ux',
    title: '委派体验对标 subagent',
    detail: '让其抓取竞品事件流，输出 badge token 建议。',
    targetSubagentId: 'immersive-ux',
    accent: 'Parallel · 2 slots',
  },
  {
    id: 'delegate-tooling',
    title: '委派组件验证 subagent',
    detail: '把 ToolOutputCard 的折叠逻辑跑一遍并截图。',
    targetSubagentId: 'tooling-audit',
    accent: 'Parallel · 2 slots',
  },
  {
    id: 'delegate-replay',
    title: '拉起回放脚本 subagent',
    detail: '结合滚动锚点，为演示版准备自动播放脚本。',
    targetSubagentId: 'replay-script',
    accent: 'Serial · 1 slot',
  },
];

const finalToolPreview: FinalToolPreview = {
  title: 'FINAL · 汇总报告',
  description: '聚合全部 subagent 输出，生成对外沟通可直接引用的总结。',
  expectedResult:
    '整理出实时工具流的自动滚动策略，并给出 badge token 的映射与回放脚本建议。',
  highlights: [
    { label: '迭代', value: '2' },
    { label: 'Subagent', value: '3 并行' },
    { label: 'Tokens', value: '30.8K' },
  ],
};

const subagentTitleMap: Record<string, string> = Object.fromEntries(
  orchestrationSubagents.map((task) => [task.id, task.title]),
);

const mockEvents: AnyAgentEvent[] = [
  {
    ...baseEventContext,
    event_type: 'workflow.input.received',
    timestamp: atOffset(0),
    agent_level: 'core',
    task: '调研自动化代理的实时回传方案，并输出总结报告。',
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.started',
    timestamp: atOffset(18),
    agent_level: 'core',
    iteration: 1,
    total_iters: 3,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.started',
    timestamp: atOffset(22),
    agent_level: 'core',
    iteration: 1,
    call_id: 'think-core-1',
    tool_name: 'think',
    arguments: {
      goal: '梳理研究切片与待委派的 subagent',
    },
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.progress',
    timestamp: atOffset(26),
    agent_level: 'core',
    call_id: 'think-core-1',
    chunk: '需要先调研终端流展示，再验证组件状态与回放脚本。\n',
    is_complete: false,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.completed',
    timestamp: atOffset(30),
    agent_level: 'core',
    call_id: 'think-core-1',
    tool_name: 'think',
    result: '整理出体验对标 + 组件验证 + 回放脚本三条路线。',
    duration: 2000,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.started',
    timestamp: atOffset(34),
    agent_level: 'core',
    iteration: 1,
    call_id: 'delegate-core-1',
    tool_name: 'delegate_subagents',
    arguments: {
      subtasks: 2,
      focus: ['体验对标', '组件验证'],
    },
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.completed',
    timestamp: atOffset(37),
    agent_level: 'core',
    call_id: 'delegate-core-1',
    tool_name: 'delegate_subagents',
    result: '激活 2 个 subagent，并行负责体验调研与组件输出校验。',
    duration: 1400,
  },
  {
    ...subagentOneContext,
    event_type: 'workflow.tool.started',
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
    event_type: 'workflow.tool.progress',
    timestamp: atOffset(50),
    call_id: 'sub-call-1',
    chunk: '📚 收集 GitHub Copilot 与 Cursor 控制台的排版策略...\n',
    is_complete: false,
  },
  {
    ...subagentOneContext,
    event_type: 'workflow.tool.progress',
    timestamp: atOffset(52),
    call_id: 'sub-call-1',
    chunk: '强调「工具列 + 时间线」分屏，加上高对比 badge。\n',
    is_complete: true,
  },
  {
    ...subagentOneContext,
    event_type: 'workflow.tool.completed',
    timestamp: atOffset(54),
    call_id: 'sub-call-1',
    tool_name: 'web_search',
    result: '归纳出 5 条关于多窗口事件回传的模式可供采用。',
    duration: 3200,
  },
  {
    ...subagentTwoContext,
    event_type: 'workflow.tool.started',
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
    event_type: 'workflow.tool.progress',
    timestamp: atOffset(66),
    call_id: 'sub-call-2',
    chunk: '比对 props 传递链路，确认 subtask metadata 是否完整...',
    is_complete: false,
  },
  {
    ...subagentTwoContext,
    event_type: 'workflow.tool.progress',
    timestamp: atOffset(69),
    call_id: 'sub-call-2',
    chunk: '需要在 mock 数据中加入 parent_task_id 与并行系数。',
    is_complete: true,
  },
  {
    ...subagentTwoContext,
    event_type: 'workflow.tool.completed',
    timestamp: atOffset(72),
    call_id: 'sub-call-2',
    tool_name: 'code_search',
    result: '确认 EventLine 组件渲染子任务标题，建议补测试覆盖。',
    duration: 3600,
  },
  {
    ...subagentOneContext,
    event_type: 'workflow.result.final',
    timestamp: atOffset(73),
    final_answer: '完成对标调研，输出 badge 体系建议。',
    total_iterations: 1,
    total_tokens: 2400,
    stop_reason: 'completed',
    duration: 4200,
  },
  {
    ...subagentTwoContext,
    event_type: 'workflow.result.final',
    timestamp: atOffset(74),
    final_answer: '补齐子任务工具事件 Mock，确保 UI 预览对齐。',
    total_iterations: 1,
    total_tokens: 2100,
    stop_reason: 'completed',
    duration: 4000,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.completed',
    timestamp: atOffset(70),
    agent_level: 'core',
    iteration: 1,
    tokens_used: 8240,
    tools_run: 1,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.output.delta',
    timestamp: atOffset(74),
    created_at: atOffset(74),
    agent_level: 'core',
    iteration: 1,
    delta: '第一轮总结：搜索结果显示领先团队都实现了逐 token 更新。',
    final: false,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.output.delta',
    timestamp: atOffset(76),
    created_at: atOffset(76),
    agent_level: 'core',
    iteration: 1,
    delta: '我们需要在终端流中引入渐进式渲染来降低用户等待。',
    final: true,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.started',
    timestamp: atOffset(78),
    agent_level: 'core',
    iteration: 2,
    total_iters: 3,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.started',
    timestamp: atOffset(82),
    agent_level: 'core',
    iteration: 2,
    call_id: 'think-core-2',
    tool_name: 'think',
    arguments: {
      goal: '汇总第一轮洞察并确认收尾动作',
    },
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.progress',
    timestamp: atOffset(84),
    agent_level: 'core',
    call_id: 'think-core-2',
    chunk: '需要拉起回放脚本 subagent，准备最终合成。\n',
    is_complete: false,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.completed',
    timestamp: atOffset(86),
    agent_level: 'core',
    call_id: 'think-core-2',
    tool_name: 'think',
    result: '确认第二轮聚焦回放脚本，待全部子任务完成再触发 Final 工具。',
    duration: 1200,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.started',
    timestamp: atOffset(86),
    agent_level: 'core',
    iteration: 2,
    call_id: 'delegate-core-2',
    tool_name: 'delegate_subagent',
    arguments: {
      subtasks: 1,
      focus: ['回放脚本'],
    },
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.completed',
    timestamp: atOffset(90),
    agent_level: 'core',
    call_id: 'delegate-core-2',
    tool_name: 'delegate_subagent',
    result: '拉起回放脚本 subagent，跟踪滚动锚点并生成回放脚本。',
    duration: 1800,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.completed',
    timestamp: atOffset(112),
    agent_level: 'core',
    iteration: 2,
    tokens_used: 9620,
    tools_run: 1,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.output.delta',
    timestamp: atOffset(120),
    created_at: atOffset(120),
    agent_level: 'core',
    iteration: 2,
    delta: '第二轮调研补充了浏览器端的实时回传模式，建议结合。',
    final: false,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.node.output.delta',
    timestamp: atOffset(132),
    created_at: atOffset(132),
    agent_level: 'core',
    iteration: 2,
    delta: '最终方案：同时保留工具状态区与逐字增长的主回答气泡。',
    final: true,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.started',
    timestamp: atOffset(134),
    agent_level: 'core',
    iteration: 2,
    call_id: 'final-call',
    tool_name: 'final_report',
    arguments: {
      sources: ['immersive-ux', 'tooling-audit', 'replay-script'],
      mode: 'synthesis',
    },
  },
  {
    ...baseEventContext,
    event_type: 'workflow.tool.completed',
    timestamp: atOffset(140),
    agent_level: 'core',
    call_id: 'final-call',
    tool_name: 'final_report',
    result:
      '综合子任务输出，得出自动滚动策略 + badge token 对齐方式，并给出回放脚本步骤。',
    duration: 2600,
  },
  {
    ...baseEventContext,
    event_type: 'workflow.result.final',
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

export default function ConsolePreviewContent() {
  return (
    <div className="min-h-screen bg-slate-100 px-6 py-10">
      <div className="mx-auto flex max-w-4xl flex-col gap-8">
        <header className="space-y-2">
          <p className="text-[10px] font-semibold text-slate-400">
            Dev Preview · Mocked Data
          </p>
          <h1 className="text-2xl font-semibold text-slate-900">
            多轮工具调用事件流（Phase 4 无框排版示例）
          </h1>
          <p className="max-w-2xl text-sm text-slate-600">
            该页面通过静态数据模拟「主 Agent 只使用 Think 与 Subagent 工具」的编排流程：主流程只做拆解、委派与 Final 汇总，所有执行细节都在子任务中自动折叠呈现。
          </p>
        </header>

        <section className="rounded-3xl bg-white/80 p-6 ring-1 ring-white/70">
          <OrchestrationBoard />
        </section>

        <section className="rounded-3xl bg-white/60 p-6 ring-1 ring-white/70">
          <TerminalOutput
            events={mockEvents}
            isConnected
            isReconnecting={false}
            error={null}
            reconnectAttempts={0}
            onReconnect={() => {}}
          />
        </section>

        <section className="rounded-3xl bg-white/80 p-6 ring-1 ring-white/70">
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
                <div className="text-[11px] font-semibold text-slate-400">
                  Input
                </div>
                <p className="mt-2 text-sm leading-6 text-slate-700">
                  {previewInput.primary}
                </p>
              </div>

              {previewInput.supporting.length > 0 && (
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
                <div className="text-[11px] font-semibold text-slate-300">
                  Output
                </div>
              </div>

              <div className="space-y-4">
                {previewOutputs.map((output) => (
                  <div
                    key={`${output.iteration}-${output.content}`}
                    className="rounded-xl border border-white/10 bg-white/5 p-4"
                  >
                    <div className="text-[11px] font-medium text-white/70">
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

function OrchestrationBoard() {
  const runningCount = orchestrationSubagents.filter(
    (task) => task.status !== 'completed',
  ).length;
  const isRunning = runningCount > 0;
  const allDone = runningCount === 0;

  return (
    <div className="space-y-6">
      <header className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <p className="text-[10px] font-semibold text-slate-400">
            Multi-Agent Orchestration
          </p>
          <h2 className="text-xl font-semibold text-slate-900">主 Agent 调度板</h2>
        </div>
        <StatusBadge tone={isRunning ? 'info' : 'success'}>
          {isRunning ? `运行中 · ${runningCount} 个子任务` : '所有子任务完成'}
        </StatusBadge>
      </header>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.55fr)_minmax(0,1fr)]">
        <MainAgentColumn
          isRunning={isRunning}
          thinkMoments={mainAgentThinkMoments}
          delegations={mainAgentDelegations}
          subagentTitles={subagentTitleMap as Record<string, string>}
        />
        <SubagentColumn subagents={orchestrationSubagents} />
      </div>

      <FinalToolCard
        tool={finalToolPreview}
        state={allDone ? 'ready' : 'waiting'}
        waitingCount={runningCount}
      />
    </div>
  );
}

function MainAgentColumn({
  isRunning,
  thinkMoments,
  delegations,
  subagentTitles,
}: {
  isRunning: boolean;
  thinkMoments: ThinkMoment[];
  delegations: DelegationMoment[];
  subagentTitles: Record<string, string>;
}) {
  return (
    <div className="space-y-4 rounded-3xl border border-slate-200/70 bg-white/90 p-5">
      {isRunning && <ThinkingStatusCard />}
      <div className="space-y-4">
        {thinkMoments.map((moment) => (
          <MainAgentEvent
            key={moment.id}
            variant="think"
            title="思考"
            description={moment.content}
            accent={moment.accent}
          />
        ))}
        {delegations.map((delegation) => (
          <MainAgentEvent
            key={delegation.id}
            variant="delegate"
            title={delegation.title}
            description={`${delegation.detail} · 目标：${subagentTitles[delegation.targetSubagentId]}`}
            accent={delegation.accent}
          />
        ))}
      </div>
    </div>
  );
}

function MainAgentEvent({
  variant,
  title,
  description,
  accent,
}: {
  variant: 'think' | 'delegate';
  title: string;
  description: string;
  accent: string;
}) {
  const isThinking = variant === 'think';
  return (
    <div className="flex gap-3">
      <span
        className={`mt-0.5 inline-flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-2xl border text-base ${
          isThinking
            ? 'border-indigo-100 bg-indigo-50 text-indigo-600'
            : 'border-emerald-100 bg-emerald-50 text-emerald-600'
        }`}
        aria-hidden
      >
        {isThinking ? <Brain className="h-5 w-5" /> : <Sparkles className="h-5 w-5" />}
      </span>
      <div className="min-w-0 space-y-1">
        <p className="text-sm font-semibold text-slate-900">{title}</p>
        <p className="text-xs text-slate-600">{description}</p>
        <p className="text-[10px] text-slate-400">{accent}</p>
      </div>
    </div>
  );
}

function ThinkingStatusCard() {
  return (
    <div className="relative overflow-hidden rounded-2xl border border-indigo-100 bg-gradient-to-r from-indigo-50 via-white to-indigo-50 p-4">
      <div className="flex items-center gap-3">
        <span className="inline-flex h-8 w-8 items-center justify-center rounded-xl border border-indigo-100 bg-white text-indigo-600">
          <Brain className="h-4 w-4" />
        </span>
        <div>
          <p className="text-sm font-semibold text-slate-900">主 Agent 正在思考下一步</p>
          <p className="text-xs text-slate-500">等待子任务回传并准备最终汇总</p>
        </div>
        <span className="workflow-node-output-delta-pill ml-auto inline-flex items-center gap-1 rounded-full border border-indigo-200 bg-indigo-500/10 px-3 py-1 text-xs font-semibold text-indigo-700">
          <Sparkles className="h-3.5 w-3.5" />
          <span>思考中</span>
        </span>
      </div>
      <style jsx>{`
        .workflow-node-output-delta-pill {
          position: relative;
          overflow: hidden;
        }
        .workflow-node-output-delta-pill::after {
          content: '';
          position: absolute;
          inset: 0;
          background: linear-gradient(90deg, transparent, rgba(255, 255, 255, 0.9), transparent);
          transform: translateX(-100%);
          animation: shimmer 1.6s linear infinite;
        }
        .workflow-node-output-delta-pill span,
        .workflow-node-output-delta-pill svg {
          position: relative;
          z-index: 1;
        }
        @keyframes shimmer {
          100% {
            transform: translateX(100%);
          }
        }
        @media (prefers-reduced-motion: reduce) {
          .workflow-node-output-delta-pill::after {
            animation: none;
          }
        }
      `}</style>
    </div>
  );
}

function SubagentColumn({ subagents }: { subagents: SubagentMission[] }) {
  return (
    <div className="space-y-4">
      {subagents.map((subagent) => (
        <SubagentCard key={subagent.id} task={subagent} />
      ))}
    </div>
  );
}

function SubagentCard({ task }: { task: SubagentMission }) {
  const isCompleted = task.status === 'completed';
  const statusLabel =
    task.status === 'completed'
      ? '已完成'
      : task.status === 'running'
      ? '执行中'
      : '待启动';

  return (
    <article className="space-y-4 rounded-3xl border border-slate-200/80 bg-white/90 p-5">
      <header className="flex items-start gap-3">
        <span
          className={`mt-1 inline-flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border-2 ${
            isCompleted ? 'border-emerald-500 bg-emerald-500' : 'border-slate-300'
          }`}
          aria-hidden
        >
          {isCompleted ? <CheckCircle2 className="h-3 w-3 text-white" /> : null}
        </span>
        <div className="min-w-0 flex-1 space-y-1">
          <p className="text-base font-semibold text-slate-900">{task.title}</p>
          <p className="text-xs text-slate-500">{task.preview}</p>
          <p className="text-sm text-slate-600">{task.outcome}</p>
        </div>
        <StatusBadge
          tone={
            task.status === 'completed'
              ? 'success'
              : task.status === 'running'
              ? 'info'
              : 'muted'
          }
        >
          {statusLabel}
        </StatusBadge>
      </header>

      <CollapsibleSection title="思考过程">
        <ol className="space-y-2 text-sm text-slate-600">
          {task.outputDeltas.map((step, index) => (
            <li key={`${task.id}-think-${index}`} className="flex gap-2">
              <span className="text-xs font-semibold text-slate-400">{index + 1}.</span>
              <span className="flex-1">{step}</span>
            </li>
          ))}
        </ol>
      </CollapsibleSection>

      <CollapsibleSection title="工具执行">
        <div className="space-y-4">
          {task.tools.map((tool, index) => {
            const isLast = index === task.tools.length - 1;
            const tone: StatusTone =
              tool.status === 'completed'
                ? 'success'
                : tool.status === 'running'
                ? 'info'
                : 'warning';
            return (
              <div key={tool.id} className="relative pl-6">
                {!isLast && (
                  <span className="absolute left-[7px] top-5 h-full w-px bg-slate-200" aria-hidden />
                )}
                <span
                  className={`absolute left-0 top-4 inline-flex h-3.5 w-3.5 items-center justify-center rounded-full border-2 ${
                    tone === 'success'
                      ? 'border-emerald-400 bg-emerald-400'
                      : tone === 'info'
                      ? 'border-sky-300 bg-white'
                      : 'border-amber-300 bg-white'
                  }`}
                  aria-hidden
                >
                  {tone === 'success' ? (
                    <CheckCircle2 className="h-2.5 w-2.5 text-white" />
                  ) : null}
                </span>
                <div className="rounded-2xl border border-slate-200/70 bg-slate-50/80 px-3.5 py-2.5">
                  <div className="flex items-center gap-2">
                    <p className="text-sm font-semibold text-slate-900">{tool.label}</p>
                    <StatusBadge tone={tone}>
                      {tone === 'success'
                        ? '完成'
                        : tone === 'info'
                        ? '执行中'
                        : '等待'}
                    </StatusBadge>
                  </div>
                  <p className="mt-1 text-xs text-slate-500">{tool.summary}</p>
                  <div className="mt-2 flex items-center justify-between text-[11px] text-slate-500">
                    <p>{tool.detail}</p>
                    <span className="font-semibold text-slate-400">
                      {tool.duration}
                    </span>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </CollapsibleSection>
    </article>
  );
}

function CollapsibleSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <details className="group rounded-2xl border border-slate-200/70 bg-white/80">
      <summary className="flex cursor-pointer items-center justify-between gap-2 px-4 py-3 text-sm font-semibold text-slate-700">
        <span>{title}</span>
        <ChevronDown className="h-4 w-4 transition group-open:rotate-180" />
      </summary>
      <div className="border-t border-slate-100 px-4 py-4">{children}</div>
    </details>
  );
}

function FinalToolCard({
  tool,
  state,
  waitingCount,
}: {
  tool: FinalToolPreview;
  state: 'ready' | 'waiting';
  waitingCount: number;
}) {
  const isReady = state === 'ready';
  return (
    <div className="space-y-4 rounded-3xl border border-indigo-100 bg-gradient-to-r from-indigo-50 via-white to-emerald-50 p-5">
      <div className="flex flex-wrap items-center gap-3">
        <span className="inline-flex h-10 w-10 items-center justify-center rounded-2xl bg-white text-indigo-600">
          <Sparkles className="h-5 w-5" />
        </span>
        <div className="min-w-0 flex-1">
          <p className="text-[10px] font-semibold text-indigo-500">
            Final 工具
          </p>
          <p className="text-lg font-semibold text-slate-900">{tool.title}</p>
        </div>
        <StatusBadge tone={isReady ? 'success' : 'warning'}>
          {isReady ? '已生成' : `等待 ${waitingCount} 个子任务`}
        </StatusBadge>
      </div>
      <p className="text-sm text-slate-600">{tool.description}</p>
      <div className="rounded-2xl border border-white/60 bg-white/80 p-4">
        <p className="text-[10px] font-semibold text-slate-400">
          {isReady ? '最终结果' : '预期输出'}
        </p>
        <p className="mt-2 text-sm leading-6 text-slate-800">{tool.expectedResult}</p>
      </div>
      <dl className="grid gap-4 sm:grid-cols-3">
        {tool.highlights.map((highlight) => (
          <div key={highlight.label} className="rounded-2xl border border-white/50 bg-white/70 p-3 text-center">
            <dt className="text-[10px] font-semibold text-slate-400">
              {highlight.label}
            </dt>
            <dd className="text-lg font-semibold text-slate-900">{highlight.value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function StatusBadge({ tone, children }: { tone: StatusTone; children: ReactNode }) {
  const toneClass: Record<StatusTone, string> = {
    muted: 'border-slate-200 bg-slate-100 text-slate-600',
    info: 'border-sky-200 bg-sky-50 text-sky-700',
    success: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    warning: 'border-amber-200 bg-amber-50 text-amber-700',
  } as const;
  return (
    <span className={`inline-flex items-center gap-1 rounded-full border px-3 py-1 text-xs font-semibold ${toneClass[tone]}`}>
      {children}
    </span>
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
  const taskComplete = findEvent(events, 'workflow.result.final');

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

function buildPreviewInput(events: AnyAgentEvent[]): { primary: string; supporting: string[]; summary: string | null } {
  const userTask = findEvent(events, 'workflow.input.received');
  const taskComplete = findEvent(events, 'workflow.result.final');

  return {
    primary:
      userTask?.task ?? '暂无输入，等待用户任务。',
    supporting: [],
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
    if (event.event_type !== 'workflow.node.output.delta') {
      return;
    }

    const assistantEvent = event as WorkflowNodeOutputDeltaEvent;
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
