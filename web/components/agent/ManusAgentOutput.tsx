'use client';

import { useState, useEffect, useMemo } from 'react';
import { AnyAgentEvent, ResearchPlan as ResearchPlanType, ResearchPlanEvent } from '@/lib/types';
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
  const memoryStats = useMemoryStats();
  const timelineSteps = useTimelineSteps(events);
  const toolOutputs = useToolOutputs(events);
  const [activeTab, setActiveTab] = useState<'timeline' | 'events' | 'document'>('timeline');
  const [documentViewMode, setDocumentViewMode] = useState<ViewMode>('default');

  // Extract research plan from events
  const researchPlan = useMemo(() => {
    const planEvent = events.find((e) => e.event_type === 'research_plan') as ResearchPlanEvent | undefined;
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
    const taskComplete = events.find((e) => e.event_type === 'task_complete');
    if (!taskComplete) return null;

    const e = taskComplete as any;
    return {
      id: 'task-result',
      title: 'Task Result',
      content: e.final_answer || 'Task completed successfully',
      type: 'markdown',
      timestamp: new Date(taskComplete.timestamp).getTime(),
      metadata: {
        iterations: e.total_iterations,
        tokens: e.total_tokens,
        stop_reason: e.stop_reason,
      },
    };
  }, [events]);

  return (
    <div className="space-y-6">
      {/* Connection status */}
      <div className="glass-card p-4 rounded-xl shadow-soft flex items-center justify-between">
        <div className="flex items-center gap-4">
          <h2 className="text-lg font-bold bg-gradient-to-r from-gray-900 to-gray-700 bg-clip-text text-transparent">
            Agent Output
          </h2>
          {/* Memory usage indicator */}
          <div className="text-xs text-gray-500 font-mono">
            {memoryStats.eventCount} events ({Math.round(memoryStats.estimatedBytes / 1024)}KB)
          </div>
        </div>
        <ConnectionStatus
          connected={isConnected}
          reconnecting={isReconnecting}
          error={error}
          reconnectAttempts={reconnectAttempts}
          onReconnect={onReconnect}
        />
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

      {/* Main content area with tabs */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left pane: Timeline/Events */}
        <div className="lg:col-span-2 space-y-4">
          <Card className="glass-card p-4">
            <Tabs value={activeTab} onValueChange={(v) => setActiveTab(v as any)}>
              <TabsList className="grid w-full grid-cols-3 mb-4">
                <TabsTrigger value="timeline" className="flex items-center gap-2">
                  <Activity className="h-4 w-4" />
                  Timeline
                </TabsTrigger>
                <TabsTrigger value="events" className="flex items-center gap-2">
                  <LayoutGrid className="h-4 w-4" />
                  Events
                </TabsTrigger>
                <TabsTrigger value="document" className="flex items-center gap-2">
                  <FileText className="h-4 w-4" />
                  Document
                </TabsTrigger>
              </TabsList>

              <TabsContent value="timeline" className="mt-0">
                {timelineSteps.length > 0 ? (
                  <ResearchTimeline steps={timelineSteps} />
                ) : (
                  <div className="text-center py-12 text-gray-500">
                    <Activity className="h-12 w-12 mx-auto mb-2 text-gray-300" />
                    <p>No timeline steps yet</p>
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
                  <div className="text-center py-12 text-gray-500">
                    <FileText className="h-12 w-12 mx-auto mb-2 text-gray-300" />
                    <p>Document will appear when task completes</p>
                  </div>
                )}
              </TabsContent>
            </Tabs>
          </Card>
        </div>

        {/* Right pane: Computer View */}
        <div className="lg:col-span-1">
          <WebViewport outputs={toolOutputs} />
        </div>
      </div>
    </div>
  );
}
