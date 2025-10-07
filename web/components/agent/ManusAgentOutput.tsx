'use client';

import { useState, useEffect, useMemo } from 'react';
import { AnyAgentEvent, ResearchPlan as ResearchPlanType, ResearchPlanEvent } from '@/lib/types';
import { isTaskCompleteEvent, isResearchPlanEvent } from '@/lib/typeGuards';
import { ConnectionStatus } from './ConnectionStatus';
import { VirtualizedEventList } from './VirtualizedEventList';
import { ResearchPlanCard } from './ResearchPlanCard';
import { ResearchTimeline } from './ResearchTimeline';
import { WebViewport } from './WebViewport';
import { DocumentCanvas, DocumentContent, ViewMode } from './DocumentCanvas';
import { useMemoryStats } from '@/hooks/useAgentStreamStore';
import { useTimelineSteps } from '@/hooks/useTimelineSteps';
import { useToolOutputs } from '@/hooks/useToolOutputs';
import { usePlanApproval } from '@/hooks/usePlanApproval';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { FileText, Activity, Monitor, LayoutGrid } from 'lucide-react';

interface ManusAgentOutputProps {
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

export function ManusAgentOutput({
  events,
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
  sessionId,
  taskId,
  autoApprovePlan = false,
}: ManusAgentOutputProps) {
  // const memoryStats = useMemoryStats();
  const timelineSteps = useTimelineSteps(events);
  const toolOutputs = useToolOutputs(events);
  const [activeTab, setActiveTab] = useState<'timeline' | 'events' | 'document'>('timeline');
  const [documentViewMode, setDocumentViewMode] = useState<ViewMode>('default');

  // Extract research plan from events
  const researchPlan = useMemo(() => {
    const planEvent = events.find(isResearchPlanEvent);
    if (!planEvent) return null;

    return {
      goal: 'Research and execute task', // Could be extracted from task_analysis event
      steps: planEvent.plan_steps,
      estimated_tools: [], // Could be inferred from past events
      estimated_iterations: planEvent.estimated_iterations,
    };
  }, [events]);

  // Plan approval flow
  const {
    state: planState,
    currentPlan,
    isSubmitting,
    handlePlanGenerated,
    handleApprove,
    handleModify,
    handleCancel,
  } = usePlanApproval({
    sessionId,
    taskId,
    onApproved: () => {
      console.log('Plan approved, execution started');
    },
    onRejected: () => {
      console.log('Plan rejected');
    },
  });

  // Auto-generate plan when research_plan event arrives
  useEffect(() => {
    if (researchPlan && planState === 'idle') {
      handlePlanGenerated(researchPlan);
    }
  }, [researchPlan, planState]);

  // Auto-approve if enabled
  useEffect(() => {
    if (autoApprovePlan && planState === 'awaiting_approval' && currentPlan) {
      handleApprove();
    }
  }, [autoApprovePlan, planState, currentPlan]);

  // Build document from task completion
  const document = useMemo((): DocumentContent | null => {
    const taskComplete = events.find(isTaskCompleteEvent);
    if (!taskComplete) return null;

    return {
      id: 'task-result',
      title: 'Task Result',
      content: taskComplete.final_answer || 'Task completed successfully',
      type: 'markdown',
      timestamp: new Date(taskComplete.timestamp).getTime(),
      metadata: {
        iterations: taskComplete.total_iterations,
        tokens: taskComplete.total_tokens,
        stop_reason: taskComplete.stop_reason,
      },
    };
  }, [events]);

  return (
    <div className="space-y-6">
      {/* Connection status - Manus minimal style */}
      <div className="manus-section">
        <div className="flex items-center justify-between">
          <h2 className="text-base font-semibold tracking-tight">
            Agent Output
          </h2>
          <ConnectionStatus
            connected={isConnected}
            reconnecting={isReconnecting}
            error={error}
            reconnectAttempts={reconnectAttempts}
            onReconnect={onReconnect}
          />
        </div>
      </div>

      {/* Plan approval card (if awaiting approval) */}
      {planState === 'awaiting_approval' && currentPlan && !autoApprovePlan && (
        <ResearchPlanCard
          plan={currentPlan}
          loading={isSubmitting}
          onApprove={handleApprove}
          onModify={handleModify}
          onCancel={handleCancel}
        />
      )}

      {/* Plan summary (after approval) */}
      {planState === 'approved' && currentPlan && (
        <ResearchPlanCard plan={currentPlan} readonly />
      )}

      {/* Main content area with tabs - Manus minimal style */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left pane: Timeline/Events */}
        <div className="lg:col-span-2 space-y-4">
          <div className="manus-card">
            <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as 'timeline' | 'events' | 'document')}>
              <TabsList className="grid w-full grid-cols-3 mb-4 bg-muted p-1 rounded-md">
                <TabsTrigger value="timeline" className="text-xs">
                  <Activity className="h-3 w-3 mr-1.5" />
                  Timeline
                </TabsTrigger>
                <TabsTrigger value="events" className="text-xs">
                  <LayoutGrid className="h-3 w-3 mr-1.5" />
                  Events
                </TabsTrigger>
                <TabsTrigger value="document" className="text-xs">
                  <FileText className="h-3 w-3 mr-1.5" />
                  Document
                </TabsTrigger>
              </TabsList>

              <TabsContent value="timeline" className="mt-0">
                {timelineSteps.length > 0 ? (
                  <ResearchTimeline steps={timelineSteps} />
                ) : (
                  <div className="text-center py-12 text-muted-foreground">
                    <Activity className="h-8 w-8 mx-auto mb-2 opacity-20" />
                    <p className="text-xs">No timeline steps yet</p>
                  </div>
                )}
              </TabsContent>

              <TabsContent value="events" className="mt-0">
                <VirtualizedEventList events={events} autoScroll={true} />
              </TabsContent>

              <TabsContent value="document" className="mt-0">
                {document ? (
                  <DocumentCanvas
                    document={document}
                    initialMode={documentViewMode}
                  />
                ) : (
                  <div className="text-center py-12 text-muted-foreground">
                    <FileText className="h-8 w-8 mx-auto mb-2 opacity-20" />
                    <p className="text-xs">Document will appear when task completes</p>
                  </div>
                )}
              </TabsContent>
            </Tabs>
          </div>
        </div>

        {/* Right pane: Computer View */}
        <div className="lg:col-span-1">
          <div className="manus-card p-4">
            <h3 className="text-xs font-semibold mb-3 tracking-tight">Tool Outputs</h3>
            {toolOutputs.length > 0 ? (
              <WebViewport outputs={toolOutputs} />
            ) : (
              <div className="text-center py-8 text-muted-foreground">
                <Monitor className="h-8 w-8 mx-auto mb-2 opacity-20" />
                <p className="text-xs">No tool outputs yet</p>
                <p className="text-xs mt-1">Results will appear here as tools execute</p>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
