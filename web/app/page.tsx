'use client';

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { TaskInput } from '@/components/agent/TaskInput';
import { TerminalOutput } from '@/components/agent/TerminalOutput';
import { ConnectionStatus } from '@/components/agent/ConnectionStatus';
import { ResearchTimeline } from '@/components/agent/ResearchTimeline';
import { useTaskExecution } from '@/hooks/useTaskExecution';
import { useAgentEventStream } from '@/hooks/useAgentEventStream';
import { useSessionStore } from '@/hooks/useSessionStore';
import { toast } from '@/components/ui/toast';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';

function HomePageContent() {
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [taskId, setTaskId] = useState<string | null>(null);
  const [focusedStepId, setFocusedStepId] = useState<string | null>(null);
  const outputRef = useRef<HTMLDivElement>(null);
  const highlightTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const highlightedElementRef = useRef<HTMLElement | null>(null);
  const searchParams = useSearchParams();

  const useMockStream = useMemo(() => searchParams.get('mockSSE') === '1', [searchParams]);

  const { mutate: executeTask, isPending } = useTaskExecution();
  const {
    currentSessionId,
    setCurrentSession,
    addToHistory,
    clearCurrentSession,
    sessionHistory,
  } = useSessionStore();

  const resolvedSessionId = sessionId || currentSessionId;

  const {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    clearEvents,
    reconnect,
    addEvent,
  } = useAgentEventStream(resolvedSessionId, {
    useMock: useMockStream,
  });

  const timelineSteps = useTimelineSteps(events);
  const hasTimeline = timelineSteps.length > 0;

  // Reset focused step when available steps change
  useEffect(() => {
    if (!focusedStepId) {
      const activeStep = timelineSteps.find((step) => step.status === 'active');
      if (activeStep) {
        setFocusedStepId(activeStep.id);
      }
      return;
    }

    const exists = timelineSteps.some((step) => step.id === focusedStepId);
    if (!exists) {
      setFocusedStepId(null);
    }
  }, [timelineSteps, focusedStepId]);

  useEffect(() => {
    return () => {
      if (highlightTimeoutRef.current) {
        clearTimeout(highlightTimeoutRef.current);
      }
    };
  }, []);

  // Auto-scroll to bottom when new events arrive
  useEffect(() => {
    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight;
    }
  }, [events]);

  const handleTaskSubmit = (task: string) => {
    console.log('[HomePage] Task submitted:', task);

    // Add user task message to events (manually, since backend doesn't send it)
    const userEvent = {
      event_type: 'user_task' as const,
      timestamp: new Date().toISOString(),
      agent_level: 'core' as const,
      task,
    };
    addEvent(userEvent);

    if (useMockStream) {
      const mockSessionId = sessionId || currentSessionId || `mock-${Date.now().toString(36)}`;
      const mockTaskId = `mock-task-${Date.now().toString(36)}`;
      setSessionId(mockSessionId);
      setTaskId(mockTaskId);
      setCurrentSession(mockSessionId);
      addToHistory(mockSessionId);
      return;
    }

    executeTask(
      {
        task,
        session_id: resolvedSessionId || undefined,
        auto_approve_plan: false,
      },
      {
        onSuccess: (data) => {
          console.log('[HomePage] Task execution started:', data);
          setSessionId(data.session_id);
          setTaskId(data.task_id);
          setCurrentSession(data.session_id);
          addToHistory(data.session_id);
        },
        onError: (error) => {
          console.error('[HomePage] Task execution error:', error);
          toast.error('Task execution failed', error.message);
        },
      }
    );
  };

  const handleClear = () => {
    setSessionId(null);
    setTaskId(null);
    clearEvents();
    clearCurrentSession();
    setFocusedStepId(null);
    highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
    highlightedElementRef.current = null;
  };

  const handleSessionSelect = (id: string) => {
    if (!id) return;
    clearEvents();
    setSessionId(id);
    setTaskId(null);
    setCurrentSession(id);
    addToHistory(id);
    setFocusedStepId(null);
    highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
    highlightedElementRef.current = null;
  };

  const handleTimelineStepSelect = useCallback(
    (stepId: string) => {
      setFocusedStepId(stepId);

      const container = outputRef.current;
      if (!container) return;

      const safeId =
        typeof CSS !== 'undefined' && typeof CSS.escape === 'function'
          ? CSS.escape(stepId)
          : stepId;

      const target = container.querySelector<HTMLElement>(
        `[data-anchor-id="${safeId}"]`
      );
      if (!target) return;

      target.scrollIntoView({ behavior: 'smooth', block: 'start', inline: 'nearest' });
      target.focus({ preventScroll: true });

      highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
      highlightedElementRef.current = target;
      target.classList.add('timeline-anchor-highlight');

      if (highlightTimeoutRef.current) {
        clearTimeout(highlightTimeoutRef.current);
      }

      highlightTimeoutRef.current = setTimeout(() => {
        highlightedElementRef.current?.classList.remove('timeline-anchor-highlight');
        highlightedElementRef.current = null;
      }, 1800);
    },
    []
  );

  const isSubmitting = useMockStream ? false : isPending;
  const sessionBadge = resolvedSessionId?.slice(0, 8);

  return (
    <div className="flex flex-1">
      <div className="console-shell">
        <div className="grid flex-1 gap-6 lg:grid-cols-[320px,1fr] xl:grid-cols-[360px,1fr]">
          <aside className="console-panel flex h-full flex-col gap-6 p-6">
            <div className="space-y-2">
              <p className="console-pane-title">Manus Console</p>
              <div className="space-y-1">
                <h1 className="text-2xl font-semibold text-slate-900">Operator Dashboard</h1>
                <p className="text-sm text-slate-500">
                  {resolvedSessionId
                    ? `Active session ${sessionBadge}`
                    : '用中文或英文描述你的工作目标，开始新的研究会话。'}
                </p>
              </div>
            </div>

            <div className="flex flex-col gap-4 rounded-2xl border border-slate-100 bg-slate-50/60 p-4">
              <div className="flex items-center justify-between">
                <div className="space-y-1">
                  <p className="text-sm font-medium text-slate-600">Connection</p>
                  <p className="text-xs text-slate-400">实时状态</p>
                </div>
                <ConnectionStatus
                  connected={isConnected}
                  reconnecting={isReconnecting}
                  reconnectAttempts={reconnectAttempts}
                  error={error}
                  onReconnect={reconnect}
                />
              </div>
              {useMockStream && (
                <div
                  className="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs font-medium uppercase tracking-wide text-amber-700"
                  data-testid="mock-stream-indicator"
                >
                  Mock Stream Enabled
                </div>
              )}
              <button
                onClick={handleClear}
                className="inline-flex items-center justify-center rounded-xl bg-white px-4 py-2 text-sm font-semibold text-slate-600 shadow-sm ring-1 ring-inset ring-slate-200 transition hover:bg-slate-100"
              >
                新建对话
              </button>
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <p className="console-section-title">历史会话</p>
                <span className="text-xs text-slate-400">自动保存最近 10 个</span>
              </div>
              <div className="space-y-2 overflow-hidden rounded-2xl border border-slate-100 bg-white">
                {sessionHistory.length === 0 ? (
                  <div className="px-4 py-6 text-center text-sm text-slate-400">
                    目前还没有历史会话。
                  </div>
                ) : (
                  <ul className="max-h-64 space-y-1 overflow-y-auto px-2 py-2 console-scrollbar">
                    {sessionHistory.map((id) => {
                      const isActive = id === resolvedSessionId;
                      const prefix = id.length > 8 ? id.slice(0, 8) : id;
                      const suffix = id.slice(-4);
                      return (
                        <li key={id}>
                          <button
                            onClick={() => handleSessionSelect(id)}
                            data-testid={`session-history-${id}`}
                            aria-current={isActive ? 'true' : undefined}
                            className={`flex w-full items-center justify-between rounded-xl px-3 py-2 text-left text-sm font-medium transition ${
                              isActive
                                ? 'bg-sky-500/10 text-sky-700 ring-1 ring-inset ring-sky-400/50'
                                : 'text-slate-600 hover:bg-slate-50'
                            }`}
                          >
                            <span className="font-medium">Session {prefix}</span>
                            <span className="text-xs text-slate-400">…{suffix}</span>
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            </div>

            <div className="space-y-3 rounded-2xl border border-slate-100 bg-white p-4">
              <p className="console-section-title">快速指引</p>
              <ul className="space-y-2 text-sm text-slate-500">
                <li>• 代码生成、调试与测试</li>
                <li>• 文档撰写、研究总结</li>
                <li>• 架构分析与技术对比</li>
              </ul>
            </div>
          </aside>

          <section className="console-panel flex h-full flex-col overflow-hidden">
            <div className="flex flex-col gap-3 border-b border-slate-100 bg-white/80 px-8 py-6">
              <div className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
                <div>
                  <p className="console-pane-title">Live Thread</p>
                  <h2 className="text-xl font-semibold text-slate-900">
                    {resolvedSessionId ? `会话 ${sessionBadge}` : '新的 Manus 对话'}
                  </h2>
                </div>
                <div className="flex items-center gap-2 text-xs text-slate-400">
                  <span>自动保存</span>
                  <span className="h-1 w-1 rounded-full bg-slate-300" />
                  <span>{new Date().toLocaleTimeString()}</span>
                </div>
              </div>
              <p className="text-sm text-slate-500">
                {resolvedSessionId
                  ? '继续你的研究或提出新的请求。ALEX 将保持上下文并延续推理。'
                  : '描述你的目标，我们会生成执行计划并通过工具完成任务。'}
              </p>
            </div>

            <div className="flex min-h-[420px] flex-1 flex-col">
              <div className="flex flex-1 flex-col gap-6 lg:flex-row">
                {hasTimeline && (
                  <aside className="hidden w-72 flex-shrink-0 lg:block">
                    <div className="console-scrollbar sticky top-24 max-h-[calc(100vh-14rem)] overflow-y-auto pr-2">
                      <ResearchTimeline
                        steps={timelineSteps}
                        focusedStepId={focusedStepId}
                        onStepSelect={handleTimelineStepSelect}
                      />
                    </div>
                  </aside>
                )}

                <div className="flex min-w-0 flex-1 flex-col">
                  <div
                    ref={outputRef}
                    className="console-scrollbar flex-1 overflow-y-auto px-8 py-8"
                  >
                    {events.length === 0 ? (
                      <div className="flex h-full flex-col items-center justify-center gap-5 text-center">
                        <div className="flex items-center gap-3 rounded-full bg-slate-100 px-4 py-2 text-xs font-medium text-slate-500">
                          <span className="inline-flex h-2 w-2 rounded-full bg-sky-400" />
                          等待新的任务指令
                        </div>
                        <div className="space-y-3">
                          <p className="text-lg font-semibold text-slate-700">准备好接管你的任务</p>
                          <p className="text-sm text-slate-500">
                            提交指令后，左侧将记录历史会话，右侧展示计划、工具调用与输出结果。
                          </p>
                        </div>
                      </div>
                    ) : (
                      <TerminalOutput
                        events={events}
                        isConnected={isConnected}
                        isReconnecting={isReconnecting}
                        error={error}
                        reconnectAttempts={reconnectAttempts}
                        onReconnect={reconnect}
                        sessionId={resolvedSessionId}
                        taskId={taskId}
                      />
                    )}
                  </div>

                  <div className="border-t border-slate-100 bg-slate-50/70 px-8 py-6">
                    <TaskInput
                      onSubmit={handleTaskSubmit}
                      disabled={isSubmitting}
                      loading={isSubmitting}
                      placeholder={resolvedSessionId ? '继续对话，输入新的需求…' : '请输入你想完成的任务或问题…'}
                    />
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}

export default function HomePage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[calc(100vh-6rem)] items-center justify-center text-sm text-muted-foreground">
          Loading Manus console…
        </div>
      }
    >
      <HomePageContent />
    </Suspense>
  );
}
