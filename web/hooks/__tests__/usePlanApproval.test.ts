import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, act, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { usePlanApproval } from '../usePlanApproval';
import { apiClient } from '@/lib/api';
import { ResearchPlan } from '@/lib/types';
import React, { ReactNode } from 'react';

// Mock the API client
vi.mock('@/lib/api', () => ({
  apiClient: {
    approvePlan: vi.fn(),
  },
}));

describe('usePlanApproval', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    vi.clearAllMocks();
  });

  const wrapper = ({ children }: { children: ReactNode }) =>
    React.createElement(QueryClientProvider, { client: queryClient }, children);

  describe('State Machine', () => {
    it('should start in idle state', () => {
      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: 'task-456' }),
        { wrapper }
      );

      expect(result.current.state).toBe('idle');
      expect(result.current.currentPlan).toBeNull();
    });

    it('should transition to generating when handleGeneratePlan is called', () => {
      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: 'task-456' }),
        { wrapper }
      );

      act(() => {
        result.current.handleGeneratePlan();
      });

      expect(result.current.state).toBe('generating');
    });

    it('should transition to awaiting_approval when plan is generated', () => {
      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: 'task-456' }),
        { wrapper }
      );

      const plan: ResearchPlan = {
        plan_steps: ['Step 1', 'Step 2'],
        estimated_iterations: 5,
        estimated_time: '2 minutes',
      };

      act(() => {
        result.current.handlePlanGenerated(plan);
      });

      expect(result.current.state).toBe('awaiting_approval');
      expect(result.current.currentPlan).toEqual(plan);
    });

    it('should transition to approved when plan is approved', async () => {
      const mockApprovePlan = vi.mocked(apiClient.approvePlan);
      mockApprovePlan.mockResolvedValue({ success: true });

      const onApproved = vi.fn();
      const { result } = renderHook(
        () => usePlanApproval({
          sessionId: 'test-123',
          taskId: 'task-456',
          onApproved,
        }),
        { wrapper }
      );

      const plan: ResearchPlan = {
        plan_steps: ['Step 1', 'Step 2'],
        estimated_iterations: 5,
        estimated_time: '2 minutes',
      };

      act(() => {
        result.current.handlePlanGenerated(plan);
      });

      act(() => {
        result.current.handleApprove();
      });

      await waitFor(() => {
        expect(result.current.state).toBe('approved');
      });

      expect(mockApprovePlan).toHaveBeenCalledWith({
        session_id: 'test-123',
        task_id: 'task-456',
        approved: true,
      });
      expect(onApproved).toHaveBeenCalled();
    });

    it('should transition to rejected when plan is cancelled', async () => {
      const mockApprovePlan = vi.mocked(apiClient.approvePlan);
      mockApprovePlan.mockResolvedValue({ success: true });

      const onRejected = vi.fn();
      const { result } = renderHook(
        () => usePlanApproval({
          sessionId: 'test-123',
          taskId: 'task-456',
          onRejected,
        }),
        { wrapper }
      );

      const plan: ResearchPlan = {
        plan_steps: ['Step 1', 'Step 2'],
        estimated_iterations: 5,
        estimated_time: '2 minutes',
      };

      act(() => {
        result.current.handlePlanGenerated(plan);
      });

      act(() => {
        result.current.handleCancel();
      });

      await waitFor(() => {
        expect(result.current.state).toBe('rejected');
      });

      expect(mockApprovePlan).toHaveBeenCalledWith({
        session_id: 'test-123',
        task_id: 'task-456',
        approved: false,
      });
      expect(onRejected).toHaveBeenCalled();
    });
  });

  describe('Plan Modification', () => {
    it('should allow plan modification and submit', async () => {
      const mockApprovePlan = vi.mocked(apiClient.approvePlan);
      mockApprovePlan.mockResolvedValue({ success: true });

      const onModified = vi.fn();
      const { result } = renderHook(
        () => usePlanApproval({
          sessionId: 'test-123',
          taskId: 'task-456',
          onModified,
        }),
        { wrapper }
      );

      const originalPlan: ResearchPlan = {
        plan_steps: ['Step 1', 'Step 2'],
        estimated_iterations: 5,
        estimated_time: '2 minutes',
      };

      const modifiedPlan: ResearchPlan = {
        plan_steps: ['Modified Step 1', 'Modified Step 2', 'Step 3'],
        estimated_iterations: 7,
        estimated_time: '3 minutes',
      };

      act(() => {
        result.current.handlePlanGenerated(originalPlan);
      });

      act(() => {
        result.current.handleModify(modifiedPlan);
      });

      await waitFor(() => {
        expect(result.current.state).toBe('approved');
      });

      expect(result.current.currentPlan).toEqual(modifiedPlan);
      expect(onModified).toHaveBeenCalledWith(modifiedPlan);
      expect(mockApprovePlan).toHaveBeenCalledWith({
        session_id: 'test-123',
        task_id: 'task-456',
        approved: true,
        modified_plan: modifiedPlan,
      });
    });

    it('should update current plan when modified', () => {
      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: 'task-456' }),
        { wrapper }
      );

      const originalPlan: ResearchPlan = {
        plan_steps: ['Step 1', 'Step 2'],
        estimated_iterations: 5,
        estimated_time: '2 minutes',
      };

      const modifiedPlan: ResearchPlan = {
        plan_steps: ['Modified Step 1'],
        estimated_iterations: 3,
        estimated_time: '1 minute',
      };

      act(() => {
        result.current.handlePlanGenerated(originalPlan);
      });

      expect(result.current.currentPlan).toEqual(originalPlan);

      act(() => {
        result.current.handleModify(modifiedPlan);
      });

      expect(result.current.currentPlan).toEqual(modifiedPlan);
    });
  });

  describe('Error Handling', () => {
    it('should handle approval API errors', async () => {
      const mockApprovePlan = vi.mocked(apiClient.approvePlan);
      mockApprovePlan.mockRejectedValue(new Error('API Error'));

      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: 'task-456' }),
        { wrapper }
      );

      const plan: ResearchPlan = {
        plan_steps: ['Step 1'],
        estimated_iterations: 3,
        estimated_time: '1 minute',
      };

      act(() => {
        result.current.handlePlanGenerated(plan);
      });

      act(() => {
        result.current.handleApprove();
      });

      await waitFor(() => {
        expect(result.current.state).toBe('awaiting_approval');
      });

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        'Plan approval failed:',
        expect.any(Error)
      );

      consoleErrorSpy.mockRestore();
    });

    it('should handle missing sessionId gracefully', () => {
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      const { result } = renderHook(
        () => usePlanApproval({ sessionId: null, taskId: 'task-456' }),
        { wrapper }
      );

      const plan: ResearchPlan = {
        plan_steps: ['Step 1'],
        estimated_iterations: 3,
        estimated_time: '1 minute',
      };

      act(() => {
        result.current.handlePlanGenerated(plan);
      });

      act(() => {
        result.current.handleApprove();
      });

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        'Cannot approve plan: missing sessionId or taskId'
      );

      consoleErrorSpy.mockRestore();
    });

    it('should handle missing taskId gracefully', () => {
      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: null }),
        { wrapper }
      );

      act(() => {
        result.current.handleCancel();
      });

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        'Cannot cancel plan: missing sessionId or taskId'
      );

      consoleErrorSpy.mockRestore();
    });
  });

  describe('Reset Functionality', () => {
    it('should reset state to idle', () => {
      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: 'task-456' }),
        { wrapper }
      );

      const plan: ResearchPlan = {
        plan_steps: ['Step 1'],
        estimated_iterations: 3,
        estimated_time: '1 minute',
      };

      act(() => {
        result.current.handlePlanGenerated(plan);
      });

      expect(result.current.state).toBe('awaiting_approval');
      expect(result.current.currentPlan).toEqual(plan);

      act(() => {
        result.current.reset();
      });

      expect(result.current.state).toBe('idle');
      expect(result.current.currentPlan).toBeNull();
    });
  });

  describe('Loading State', () => {
    it('should track isSubmitting state during approval', async () => {
      const mockApprovePlan = vi.mocked(apiClient.approvePlan);
      mockApprovePlan.mockImplementation(() =>
        new Promise((resolve) => setTimeout(() => resolve({ success: true }), 100))
      );

      const { result } = renderHook(
        () => usePlanApproval({ sessionId: 'test-123', taskId: 'task-456' }),
        { wrapper }
      );

      const plan: ResearchPlan = {
        plan_steps: ['Step 1'],
        estimated_iterations: 3,
        estimated_time: '1 minute',
      };

      act(() => {
        result.current.handlePlanGenerated(plan);
      });

      expect(result.current.isSubmitting).toBe(false);

      act(() => {
        result.current.handleApprove();
      });

      // Wait for mutation to start
      await waitFor(() => {
        expect(result.current.isSubmitting).toBe(true);
      });

      await waitFor(() => {
        expect(result.current.isSubmitting).toBe(false);
      });
    });
  });
});
