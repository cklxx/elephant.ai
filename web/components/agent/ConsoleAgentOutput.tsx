'use client';

import { useState, useEffect, useMemo, useRef } from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { isResearchPlanEvent, isTaskCompleteEvent } from '@/lib/typeGuards';
import { ConnectionStatus } from './ConnectionStatus';
import { VirtualizedEventList } from './VirtualizedEventList';
import { TimelineStepList } from './TimelineStepList';
import { WebViewport } from './WebViewport';
import { DocumentCanvas, DocumentContent, ViewMode } from './DocumentCanvas';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import { useToolOutputs } from '@/hooks/useToolOutputs';
import { usePlanApproval } from '@/hooks/usePlanApproval';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { FileText, Activity, Monitor, LayoutGrid } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';
import { TaskCompleteCard } from './TaskCompleteCard';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';

interface ConsoleAgentOutputProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
  isReconnecting: boolean;
  error?: string | null;
  reconnectAttempts?: number;
  onReconnect?: () => void;
  sessionId: string | null;
  taskId: string | null;
  autoApprovePlan?: boolean;
}

export function ConsoleAgentOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  sessionId,
  taskId,
  autoApprovePlan: _autoApprovePlan = false,
}: ConsoleAgentOutputProps) {
  void _autoApprovePlan;
  const t = useTranslation();
  const timelineSteps = useTimelineSteps(events);
  const toolOutputs = useToolOutputs(events);
  const [activeTab, setActiveTab] = useState<'timeline' | 'events' | 'document'>('timeline');
  const [documentViewMode, setDocumentViewMode] = useState<ViewMode>('default');
  const [focusedStepId, setFocusedStepId] = useState<string | null>(null);
  const [hasUserSelectedStep, setHasUserSelectedStep] = useState(false);
  const lastHandledPlanKeyRef = useRef<string | null>(null);

  const hasTimeline = timelineSteps.length > 0;
  const activeTimelineStep = useMemo(
    () => timelineSteps.find((step) => step.status === 'active') ?? null,
    [timelineSteps]
  );
  const latestTimelineStep = useMemo(
    () => (timelineSteps.length > 0 ? timelineSteps[timelineSteps.length - 1] : null),
    [timelineSteps]
  );
  const fallbackTimelineStepId = activeTimelineStep?.id ?? latestTimelineStep?.id ?? null;
  const focusedTimelineStep = useMemo(
    () => (focusedStepId ? timelineSteps.find((step) => step.id === focusedStepId) ?? null : null),
    [timelineSteps, focusedStepId]
  );
  const focusedEventIndex = focusedTimelineStep?.anchorEventIndex ?? null;

  // Extract research plan from events
  const latestPlanEvent = useMemo(() => {
    for (let index = events.length - 1; index >= 0; index -= 1) {
      const candidate = events[index];
      if (isResearchPlanEvent(candidate)) {
        return candidate;
      }
    }
    return null;
  }, [events]);

  useEffect(() => {
    if (!hasTimeline) {
      if (focusedStepId !== null) {
        setFocusedStepId(null);
      }
      if (hasUserSelectedStep) {
        setHasUserSelectedStep(false);
      }
      return;
    }

    if (!hasUserSelectedStep) {
      if (fallbackTimelineStepId !== focusedStepId) {
        setFocusedStepId(fallbackTimelineStepId);
      }
      return;
    }

    const exists = timelineSteps.some((step) => step.id === focusedStepId);
    if (!exists) {
      setFocusedStepId(fallbackTimelineStepId);
      setHasUserSelectedStep(false);
    }
  }, [
    hasTimeline,
    timelineSteps,
    focusedStepId,
    hasUserSelectedStep,
    fallbackTimelineStepId,
  ]);

  const researchPlan = useMemo(() => {
    if (!latestPlanEvent) {
      return null;
    }

    return {
      goal: t('agent.output.plan.defaultGoal'),
      steps: latestPlanEvent.plan_steps,
      estimated_tools: latestPlanEvent.estimated_tools ?? [],
      estimated_iterations: latestPlanEvent.estimated_iterations,
      estimated_duration_minutes: latestPlanEvent.estimated_duration_minutes,
    };
  }, [latestPlanEvent, t]);

  const {
    state: planState,
    currentPlan,
    isSubmitting,
    handlePlanGenerated,
    handleApprove,
  } = usePlanApproval({
    sessionId,
    taskId,
  });

  useEffect(() => {
    if (!researchPlan || !latestPlanEvent) {
      return;
    }

    const planKey = `${latestPlanEvent.timestamp}:${latestPlanEvent.plan_steps.join('|')}`;
    if (lastHandledPlanKeyRef.current === planKey && planState !== 'idle') {
      return;
    }

    lastHandledPlanKeyRef.current = planKey;
    handlePlanGenerated(researchPlan);
  }, [handlePlanGenerated, latestPlanEvent, planState, researchPlan]);

  useEffect(() => {
    if (planState === 'awaiting_approval' && currentPlan && !isSubmitting) {
      handleApprove();
    }
  }, [currentPlan, handleApprove, isSubmitting, planState]);

  const latestTaskComplete = useMemo(() => {
    for (let idx = events.length - 1; idx >= 0; idx -= 1) {
      const current = events[idx];
      if (isTaskCompleteEvent(current)) {
        return current;
      }
    }
    return null;
  }, [events]);

  const latestAttachmentCount = useMemo(() => {
    if (!latestTaskComplete?.attachments) return 0;
    return Object.keys(latestTaskComplete.attachments).length;
  }, [latestTaskComplete]);

  // Build document from task completion
  const document = useMemo((): DocumentContent | null => {
    let taskComplete: AnyAgentEvent | undefined;
    for (let idx = events.length - 1; idx >= 0; idx -= 1) {
      const current = events[idx];
      if (isTaskCompleteEvent(current)) {
        taskComplete = current;
        break;
      }
    }
    if (!taskComplete || !isTaskCompleteEvent(taskComplete)) return null;

    return {
      id: 'task-result',
      title: t('agent.output.document.title'),
      content: taskComplete.final_answer || t('agent.output.document.fallback'),
      type: 'markdown',
      timestamp: new Date(taskComplete.timestamp).getTime(),
      metadata: {
        iterations: taskComplete.total_iterations,
        tokens: taskComplete.total_tokens,
        stop_reason: taskComplete.stop_reason,
      },
      attachments: taskComplete.attachments,
    };
  }, [events, t]);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="text-base font-semibold tracking-tight">
          {t('agent.output.heading')}
        </h2>
        <ConnectionStatus
          connected={isConnected}
          reconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={onReconnect}
        />
      </div>

      {latestTaskComplete && (
        <Card>
          <CardHeader className="flex flex-col gap-1 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                Final answer
              </p>
              <p className="text-xs text-muted-foreground">
                Streaming summary with resolved attachments and metrics.
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-muted-foreground">
              {latestAttachmentCount > 0 && (
                <Badge variant="outline">{latestAttachmentCount} attachments</Badge>
              )}
              {typeof latestTaskComplete.duration === 'number' && (
                <Badge variant="outline">{(latestTaskComplete.duration / 1000).toFixed(1)}s elapsed</Badge>
              )}
            </div>
          </CardHeader>
          <CardContent>
            <TaskCompleteCard event={latestTaskComplete} />
          </CardContent>
        </Card>
      )}

      {/* Main content area with tabs */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
        {/* Left pane: Timeline/Events */}
        <div className="space-y-4 lg:col-span-2">
          <Card>
            <CardContent className="p-4">
              <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'timeline' | 'events' | 'document')}>
                <TabsList className="mb-4 grid w-full grid-cols-3">
                  <TabsTrigger value="timeline" className="text-xs">
                    <Activity className="h-3 w-3 mr-1.5" />
                    {t('agent.output.tabs.timeline')}
                  </TabsTrigger>
                  <TabsTrigger value="events" className="text-xs">
                    <LayoutGrid className="h-3 w-3 mr-1.5" />
                    {t('agent.output.tabs.events')}
                  </TabsTrigger>
                  <TabsTrigger value="document" className="text-xs">
                    <FileText className="h-3 w-3 mr-1.5" />
                    {t('agent.output.tabs.document')}
                  </TabsTrigger>
                </TabsList>

                <TabsContent value="timeline" className="mt-0 space-y-4">
                  {timelineSteps.length > 0 ? (
                    <TimelineStepList
                      steps={timelineSteps}
                      focusedStepId={focusedStepId}
                      onStepSelect={(stepId) => {
                        setFocusedStepId(stepId);
                        setHasUserSelectedStep(true);
                      }}
                    />
                  ) : (
                    <div className="py-12 text-center text-muted-foreground">
                      <Activity className="mx-auto mb-2 h-8 w-8 opacity-20" />
                      <p className="text-xs">{t('agent.output.timeline.empty')}</p>
                    </div>
                  )}
                </TabsContent>

                <TabsContent value="events" className="mt-0">
                  <VirtualizedEventList
                    events={events}
                    autoScroll={!hasUserSelectedStep}
                    focusedEventIndex={focusedEventIndex}
                    onJumpToLatest={() => {
                      const targetStepId = activeTimelineStep?.id ?? latestTimelineStep?.id ?? null;
                      setFocusedStepId(targetStepId);
                      setHasUserSelectedStep(false);
                    }}
                  />
                </TabsContent>

                <TabsContent value="document" className="mt-0">
                  {document ? (
                    <DocumentCanvas
                      document={document}
                      initialMode={documentViewMode}
                    />
                  ) : (
                    <div className="py-12 text-center text-muted-foreground">
                      <FileText className="mx-auto mb-2 h-8 w-8 opacity-20" />
                      <p className="text-xs">{t('agent.output.document.empty')}</p>
                    </div>
                  )}
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>
        </div>

        {/* Right pane: Computer View */}
        <div className="lg:col-span-1">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-semibold tracking-tight">
              {t('agent.output.toolOutputs.title')}
              </CardTitle>
            </CardHeader>
            <CardContent>
              {toolOutputs.length > 0 ? (
                <WebViewport outputs={toolOutputs} />
              ) : (
                <div className="py-8 text-center text-muted-foreground">
                  <Monitor className="mx-auto mb-2 h-8 w-8 opacity-20" />
                  <p className="text-xs">{t('agent.output.toolOutputs.empty')}</p>
                  <p className="mt-1 text-xs">{t('agent.output.toolOutputs.emptyHint')}</p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
