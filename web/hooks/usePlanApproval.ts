// Plan approval hook for Manus-style plan-first workflow

import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { ResearchPlan, ApprovePlanRequest, ApprovePlanResponse } from '@/lib/types';

export type PlanApprovalState = 'idle' | 'generating' | 'awaiting_approval' | 'approved' | 'rejected';

export interface UsePlanApprovalOptions {
  sessionId: string | null;
  taskId: string | null;
  onApproved?: () => void;
  onRejected?: (reason?: string) => void;
  onModified?: (plan: ResearchPlan) => void;
}

export function usePlanApproval({
  sessionId,
  taskId,
  onApproved,
  onRejected,
  onModified,
}: UsePlanApprovalOptions) {
  const [state, setState] = useState<PlanApprovalState>('idle');
  const [currentPlan, setCurrentPlan] = useState<ResearchPlan | null>(null);

  const { mutate: submitApproval, isPending: isSubmitting } = useMutation({
    mutationFn: async (request: ApprovePlanRequest): Promise<ApprovePlanResponse> => {
      return apiClient.approvePlan(request);
    },
    onSuccess: (data, variables) => {
      if (variables.approved) {
        setState('approved');
        onApproved?.();
      } else {
        setState('rejected');
        onRejected?.(variables.rejection_reason);
      }
    },
    onError: (error: Error) => {
      console.error('Plan approval failed:', error);
      setState('awaiting_approval'); // Reset to allow retry
    },
  });

  const handleGeneratePlan = () => {
    setState('generating');
  };

  const handlePlanGenerated = (plan: ResearchPlan) => {
    setCurrentPlan(plan);
    setState('awaiting_approval');
  };

  const handleApprove = () => {
    if (!sessionId || !taskId) {
      console.error('Cannot approve plan: missing sessionId or taskId');
      return;
    }

    submitApproval({
      session_id: sessionId,
      task_id: taskId,
      approved: true,
    });
  };

  const handleModify = (updatedPlan: ResearchPlan) => {
    if (!sessionId || !taskId) {
      console.error('Cannot modify plan: missing sessionId or taskId');
      return;
    }

    setCurrentPlan(updatedPlan);
    onModified?.(updatedPlan);

    // Submit modified plan for approval
    submitApproval({
      session_id: sessionId,
      task_id: taskId,
      approved: true,
      modified_plan: updatedPlan,
    });
  };

  const handleReject = (reason?: string) => {
    if (!sessionId || !taskId) {
      console.error('Cannot reject plan: missing sessionId or taskId');
      return;
    }

    submitApproval({
      session_id: sessionId,
      task_id: taskId,
      approved: false,
      rejection_reason: reason && reason.trim().length > 0 ? reason : undefined,
    });
  };

  const handleCancel = () => {
    handleReject();
  };

  const reset = () => {
    setState('idle');
    setCurrentPlan(null);
  };

  return {
    state,
    currentPlan,
    isSubmitting,
    handleGeneratePlan,
    handlePlanGenerated,
    handleApprove,
    handleModify,
    handleReject,
    handleCancel,
    reset,
  };
}
