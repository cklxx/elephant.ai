'use client';

import { useMemo, useState, useEffect } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import type { WorkflowResultFinalEvent } from '@/lib/types';
import { TimelineStep } from '@/lib/planTypes';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import { cn } from '@/lib/utils';
import { Check, ChevronDown, Loader2, X } from 'lucide-react';
import { TaskCompleteCard } from './TaskCompleteCard';
import { isWorkflowResultFinalEvent } from '@/lib/typeGuards';

type ActionStatus = 'running' | 'done' | 'failed';

type StepAction = {
  id: string;
  status: ActionStatus;
  label: string;
  detail?: string;
};

type StepGroup = {
  step: TimelineStep;
  actions: StepAction[];
  progress?: string;
};

function shorten(text: string, max: number) {
  const trimmed = text.trim();
  if (trimmed.length <= max) return trimmed;
  return `${trimmed.slice(0, max)}…`;
}

function basename(path: string) {
  const cleaned = path.split('?')[0].split('#')[0];
  if (cleaned === '.' || cleaned === './') return '当前目录';
  if (cleaned === '..' || cleaned === '../') return '上级目录';
  const parts = cleaned.split('/').filter(Boolean);
  return parts.length ? parts[parts.length - 1] : cleaned;
}

function host(url: string) {
  try {
    const parsed = new URL(url);
    return parsed.host || url;
  } catch {
    return url;
  }
}

function isNonEmptyString(value: unknown): value is string {
  return typeof value === 'string' && value.trim().length > 0;
}

function describeToolAction(toolName: string, args: Record<string, any>): string {
  const name = toolName.toLowerCase();
  const safe = (value: unknown) => (typeof value === 'string' ? value : '');

  if (name.startsWith('mcp__chrome-devtools__') || name.includes('chrome-devtools')) {
    const url = safe(args.url) || safe(args.href) || '';
    if (name.includes('__navigate_page')) {
      const kind = safe(args.type);
      if (kind === 'reload') return '刷新页面';
      return url ? `打开 ${host(url)}` : '打开页面';
    }
    if (name.includes('__take_screenshot')) return safe(args.fullPage) ? '全页截图' : '截图';
    if (name.includes('__take_snapshot')) return '读取页面结构';
    if (name.includes('__click')) return '点击页面元素';
    if (name.includes('__hover')) return '悬停页面元素';
    if (name.includes('__drag')) return '拖拽页面元素';
    if (name.includes('__fill_form')) return '填写表单';
    if (name.includes('__fill')) {
      const value = safe(args.value);
      return value ? `输入 ${shorten(value, 18)}` : '输入内容';
    }
    if (name.includes('__press_key')) {
      const key = safe(args.key);
      return key ? `按键 ${shorten(key, 18)}` : '按键操作';
    }
    if (name.includes('__wait_for')) {
      const text = safe(args.text);
      return text ? `等待 ${shorten(text, 24)}` : '等待页面更新';
    }
    if (name.includes('__evaluate_script')) return '执行脚本';
    if (name.includes('__list_network_requests')) return '查看网络请求';
    if (name.includes('__get_network_request')) return '读取网络请求';
    if (name.includes('__list_console_messages')) return '查看控制台日志';
    if (name.includes('__get_console_message')) return '读取控制台日志';
    if (name.includes('__list_pages')) return '查看页面列表';
    if (name.includes('__select_page')) return '切换页面';
    if (name.includes('__new_page')) return url ? `打开 ${host(url)}` : '打开新页面';
    if (name.includes('__resize_page')) return '调整窗口大小';
    if (name.includes('__upload_file')) {
      const filePath = safe(args.filePath);
      return filePath ? `上传 ${basename(filePath)}` : '上传文件';
    }
    if (name.includes('__handle_dialog')) return '处理弹窗';
    return '操作浏览器';
  }

  if (name.includes('file_read')) {
    const path = safe(args.path) || safe(args.file) || '';
    return path ? `查看 ${basename(path)}` : '查看文件';
  }
  if (name.includes('file_write') || name.includes('file_edit') || name.includes('apply_patch')) {
    const path = safe(args.path) || safe(args.file) || '';
    return path ? `写入 ${basename(path)}` : '写入文件';
  }
  if (name.includes('code_search') || (name.includes('search') && (name.includes('code') || name.includes('repo')))) {
    const q = safe(args.query) || safe(args.q) || '';
    const path = safe(args.path) || '';
    if (q && path) return `搜索 ${basename(path)}：${shorten(q, 20)}`;
    if (q) return `搜索代码 ${shorten(q, 26)}`;
    return '搜索代码';
  }
  if (name.includes('artifacts_write') || name.includes('artifact_write')) {
    const path = safe(args.path) || safe(args.name) || '';
    const op = safe(args.operation);
    const verb = op === 'update' ? '更新' : op === 'create' ? '保存' : '写入';
    return path ? `${verb} ${basename(path)}` : `${verb} 附件`;
  }
  if (name.includes('artifacts_list') || name.includes('artifact_list')) {
    return '同步附件列表';
  }
  if (name === 'bash' || name.includes('shell')) {
    const cmd = safe(args.command) || safe(args.cmd) || '';
    return cmd ? `执行 ${shorten(cmd, 28)}` : '执行命令';
  }
  if (name.includes('web') && (name.includes('search') || name.includes('query'))) {
    const q = safe(args.query) || safe(args.q) || safe(args.text) || '';
    return q ? `检索 ${shorten(q, 28)}` : '检索信息';
  }
  if (name.includes('http') || name.includes('fetch') || name.includes('request') || name.includes('navigate')) {
    const url = safe(args.url) || safe(args.href) || '';
    return url ? `打开 ${host(url)}` : '打开链接';
  }
  if (name.includes('subagent') || name.includes('delegate')) {
    const prompt = safe(args.prompt) || safe(args.task) || '';
    return prompt ? `分派子任务 ${shorten(prompt, 24)}` : '分派子任务';
  }

  const url = safe(args.url) || safe(args.href) || safe(args.uri) || '';
  if (url) return `打开 ${host(url)}`;

  const path = safe(args.path) || safe(args.filePath) || safe(args.file) || safe(args.targetPath) || '';
  if (path) {
    const verb =
      name.includes('read') || name.includes('get') || name.includes('open') || name.includes('load')
        ? '查看'
        : name.includes('list') || name.includes('ls') || name.includes('dir') || name.includes('tree')
          ? '查看'
        : name.includes('write') || name.includes('edit') || name.includes('patch') || name.includes('update') || name.includes('create')
          ? '修改'
          : name.includes('delete') || name.includes('remove')
            ? '删除'
            : '处理';
    return `${verb} ${basename(path)}`;
  }

  const query = safe(args.query) || safe(args.q) || safe(args.text) || '';
  if (query) return `检索 ${shorten(query, 28)}`;

  const cmd = safe(args.command) || safe(args.cmd) || '';
  if (cmd) return `执行 ${shorten(cmd, 28)}`;

  const title = safe(args.title) || safe(args.name) || '';
  if (title) return `处理 ${shorten(title, 24)}`;

  if (name.includes('search')) return '检索信息';
  if (name.includes('list')) return '同步列表';
  if (name.includes('download')) return '下载内容';
  if (name.includes('upload')) return '上传内容';
  if (name.includes('render') || name.includes('preview')) return '生成预览';

  return '执行操作';
}

function shouldHideToolAction(toolName: string): boolean {
  const name = toolName.trim().toLowerCase();
  if (!name) return true;
  if (name === 'think' || name === 'final') return true;
  if (name.startsWith('todo_')) return true;
  return false;
}

function buildStepGroups(events: AnyAgentEvent[], steps: TimelineStep[]): StepGroup[] {
  const byId = new Map<string, StepGroup>();
  steps.forEach((step) => {
    byId.set(step.id, { step, actions: [] });
  });

  // Assign tool actions to the currently active step by event order.
  let activeStepId: string | null = null;
  const actionByCallId = new Map<string, { stepId: string; action: StepAction }>();

  for (const event of events) {
    if (event.event_type === 'workflow.node.started' && typeof (event as any).step_index === 'number') {
      activeStepId = `step-${(event as any).step_index}`;
      continue;
    }
    if (event.event_type === 'workflow.node.completed' && typeof (event as any).step_index === 'number') {
      const stepId = `step-${(event as any).step_index}`;
      if (activeStepId === stepId) {
        activeStepId = null;
      }
      continue;
    }

    if (event.event_type === 'workflow.tool.started') {
      if (!activeStepId) continue;
      const group = byId.get(activeStepId);
      if (!group) continue;
      const callId = (event as any).call_id as string | undefined;
      const toolName = (event as any).tool_name as string | undefined;
      if (!callId || !toolName) continue;
      if (shouldHideToolAction(toolName)) continue;
      const args = ((event as any).arguments ?? {}) as Record<string, any>;
      const label = describeToolAction(toolName, args);
      const action: StepAction = {
        id: callId,
        status: 'running',
        label,
      };
      group.actions.push(action);
      actionByCallId.set(callId, { stepId: activeStepId, action });
      continue;
    }

    if (event.event_type === 'workflow.tool.progress') {
      const callId = (event as any).call_id as string | undefined;
      const chunk = typeof (event as any).chunk === 'string' ? String((event as any).chunk) : '';
      const normalized = shorten(chunk.replace(/\s+/g, ' ').trim(), 88);
      if (!normalized) continue;
      if (callId) {
        const found = actionByCallId.get(callId);
        if (found) {
          found.action.detail = normalized;
          continue;
        }
      }
      if (activeStepId) {
        const group = byId.get(activeStepId);
        if (group) group.progress = normalized;
      }
      continue;
    }

    if (event.event_type === 'workflow.tool.completed') {
      const callId = (event as any).call_id as string | undefined;
      if (!callId) continue;
      const found = actionByCallId.get(callId);
      if (!found) continue;
      found.action.status = (event as any).error ? 'failed' : 'done';
      const result = (event as any).result;
      if (!found.action.detail && isNonEmptyString(result)) {
        found.action.detail = shorten(result.replace(/\s+/g, ' ').trim(), 88);
      }
    }
  }

  return steps.map((step) => byId.get(step.id) ?? { step, actions: [] });
}

export function PlanProgressView({ events }: { events: AnyAgentEvent[] }) {
  const timelineSteps = useTimelineSteps(events);

  const groups = useMemo(() => buildStepGroups(events, timelineSteps), [events, timelineSteps]);
  const activeStepId = useMemo(
    () => timelineSteps.find((step) => step.status === 'active')?.id ?? null,
    [timelineSteps],
  );
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    if (!activeStepId) return;
    setExpanded((prev) => {
      if (prev.has(activeStepId)) return prev;
      const next = new Set(prev);
      next.add(activeStepId);
      return next;
    });
  }, [activeStepId]);

  const finalEvent = useMemo(() => {
    for (let i = events.length - 1; i >= 0; i -= 1) {
      const ev = events[i];
      if (isWorkflowResultFinalEvent(ev)) return ev as WorkflowResultFinalEvent;
    }
    return null;
  }, [events]) as WorkflowResultFinalEvent | null;

  if (timelineSteps.length === 0) {
    return null;
  }

  return (
    <div className="mx-auto w-full max-w-2xl space-y-5">
      <div className="rounded-3xl border border-border/60 bg-card/70 p-4 backdrop-blur">
        <div className="space-y-1">
          {groups.map((group, idx) => {
            const step = group.step;
            const isActive = step.id === activeStepId;
            const isExpanded = expanded.has(step.id) || isActive;
            const icon = step.status === 'active'
              ? <Loader2 className="h-4 w-4 animate-spin" />
              : step.status === 'done'
                ? <Check className="h-4 w-4" />
                : step.status === 'failed'
                  ? <X className="h-4 w-4" />
                  : <span className="text-[11px] font-semibold">{idx + 1}</span>;

            const header = (
              <div className="flex items-center justify-between gap-3">
                <div className="flex min-w-0 items-center gap-3">
                  <div
                    className={cn(
                      'flex h-6 w-6 shrink-0 items-center justify-center rounded-full border',
                      step.status === 'active' && 'border-primary/60 bg-primary/10 text-primary',
                      step.status === 'done' && 'border-primary/30 bg-primary/5 text-primary',
                      step.status === 'failed' && 'border-destructive/40 bg-destructive/10 text-destructive',
                      step.status === 'planned' && 'border-border/60 text-muted-foreground',
                    )}
                    aria-hidden
                  >
                    {icon}
                  </div>
                  <div className="min-w-0">
                    <p
                      className={cn(
                        'truncate text-sm font-medium',
                        step.status === 'done' && 'text-foreground/70',
                        step.status === 'planned' && 'text-muted-foreground',
                      )}
                    >
                      {step.title}
                    </p>
                  </div>
                </div>
                <button
                  type="button"
                  className={cn(
                    'inline-flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground/70 hover:bg-muted/30 hover:text-foreground',
                    isActive && 'pointer-events-none opacity-0',
                  )}
                  aria-label="Toggle step details"
                  onClick={() => {
                    setExpanded((prev) => {
                      const next = new Set(prev);
                      if (next.has(step.id)) next.delete(step.id);
                      else next.add(step.id);
                      return next;
                    });
                  }}
                >
                  <ChevronDown className={cn('h-4 w-4 transition-transform', isExpanded && 'rotate-180')} />
                </button>
              </div>
            );

            return (
              <div key={step.id} className="rounded-2xl px-2 py-2 hover:bg-muted/10">
                {header}
                {isExpanded && (group.actions.length > 0 || (step.status !== 'planned' && step.result)) && (
                  <div className="mt-2 space-y-2 pl-9">
                    {isActive && group.progress && (
                      <div className="text-xs text-muted-foreground/80">
                        {group.progress}
                      </div>
                    )}
                    {group.actions.slice(-6).map((action) => (
                      <div
                        key={action.id}
                        className={cn(
                          'flex items-center gap-2 rounded-2xl border border-border/60 bg-background/70 px-3 py-2 text-sm',
                          action.status === 'failed' && 'border-destructive/30 bg-destructive/5',
                        )}
                      >
                        {action.status === 'running' ? (
                          <Loader2 className="h-4 w-4 animate-spin text-muted-foreground" />
                        ) : action.status === 'failed' ? (
                          <X className="h-4 w-4 text-destructive" />
                        ) : (
                          <Check className="h-4 w-4 text-primary" />
                        )}
                        <div className="min-w-0">
                          <div className="truncate text-foreground/80">
                            {action.status === 'running' ? `正在 ${action.label}` : action.label}
                          </div>
                          {action.status === 'running' && action.detail && (
                            <div className="truncate text-xs text-muted-foreground/80">
                              {action.detail}
                            </div>
                          )}
                        </div>
                      </div>
                    ))}

                    {step.status !== 'planned' && step.result && !isActive && idx !== groups.length - 1 && (
                      <div className="flex items-center gap-2 rounded-2xl border border-border/60 bg-background/50 px-3 py-2 text-sm text-muted-foreground">
                        <Check className="h-4 w-4 text-primary/70" />
                        <span className="min-w-0 truncate">{shorten(step.result, 80)}</span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {finalEvent && (
        <TaskCompleteCard event={finalEvent} />
      )}
    </div>
  );
}
