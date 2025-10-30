'use client';

import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { AnyAgentEvent } from '@/lib/types';

const baseTime = new Date('2025-10-12T08:00:00Z').getTime();

function atOffset(seconds: number) {
  return new Date(baseTime + seconds * 1000).toISOString();
}

const previewSessionId = 'preview-session';
const previewTaskId = 'preview-task';
const baseEventContext = { session_id: previewSessionId, task_id: previewTaskId } as const;

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
    event_type: 'task_complete',
    timestamp: atOffset(150),
    agent_level: 'core',
    final_answer:
      '整理出实时工具流的自动滚动策略，并给出逐步落地建议。',
    total_iterations: 2,
    total_tokens: 17860,
    stop_reason: 'end',
    duration: 150000,
  },
];

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
      </div>
    </div>
  );
}
