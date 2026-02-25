/**
 * Task Execution Hooks with React Query
 *
 * Provides mutation and query hooks for task management with:
 * - Automatic retry logic with exponential backoff
 * - Optimistic updates for better UX
 * - Error handling with detailed logging
 * - Status polling for task completion
 *
 * @example
 * ```tsx
 * const { mutate, isPending } = useTaskExecution({
 *   onSuccess: (data) => console.log('Task created:', data),
 *   onError: (error) => console.error('Task failed:', error)
 * });
 * ```
 */

import { useMutation, useQuery, UseMutationOptions, UseQueryOptions } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { getErrorLogPayload, isAPIError } from '@/lib/errors';
import { loadLLMSelection } from '@/lib/llmSelection';
import { CreateTaskRequest, CreateTaskResponse, TaskStatusResponse } from '@/lib/types';

/**
 * Options for task execution mutation
 */
interface UseTaskExecutionOptions extends Omit<
  UseMutationOptions<CreateTaskResponse, Error, CreateTaskRequest, unknown>,
  'mutationFn'
> {
  /** Enable retry on failure (default: true) */
  retry?: boolean;
  /** Maximum retry attempts (default: 3) */
  maxRetries?: number;
}

/**
 * Hook for creating and executing tasks
 *
 * Provides mutation with:
 * - Automatic retry with exponential backoff (1s, 2s, 4s)
 * - Detailed error logging
 * - Lifecycle hooks (onMutate, onSuccess, onError, onSettled)
 */
export function useTaskExecution(options: UseTaskExecutionOptions = {}) {
  const {
    retry = false,
    maxRetries = 3,
    onMutate,
    onSuccess,
    onError,
    onSettled,
    ...mutationOptions
  } = options;

  return useMutation({
    mutationFn: async (request: CreateTaskRequest): Promise<CreateTaskResponse> => {
      console.log('[useTaskExecution] Sending task request:', {
        task: request.task.slice(0, 100),
        sessionId: request.session_id,
      });

      const selection = request.llm_selection ?? loadLLMSelection();
      const response = await apiClient.createTask({
        ...request,
        ...(selection ? { llm_selection: selection } : {}),
      });

      console.log('[useTaskExecution] Task created successfully:', {
        runId: response.run_id,
        sessionId: response.session_id,
        parentRunId: response.parent_run_id ?? null,
      });

      return response;
    },

    // Retry configuration
    retry: retry ? maxRetries : false,
    retryDelay: (attemptIndex) => Math.min(1000 * Math.pow(2, attemptIndex), 10000),

    // Lifecycle hooks - pass through from options with error logging
    onMutate,
    onSuccess,
    onError: (error, variables, context, mutation) => {
      const prefix = '[useTaskExecution] Task execution failed:';
      if (isAPIError(error)) {
        console.error(prefix, getErrorLogPayload(error));
      } else {
        console.error(prefix, error);
      }
      onError?.(error, variables, context, mutation);
    },
    onSettled,

    ...mutationOptions,
  });
}

/**
 * Options for task status query
 */
interface UseTaskStatusOptions extends Omit<
  UseQueryOptions<TaskStatusResponse, Error>,
  'queryKey' | 'queryFn'
> {
  /** Polling interval in milliseconds (default: 2000) */
  pollingInterval?: number;
  /** Stop polling on these statuses (default: ['completed', 'failed', 'cancelled', 'error']) */
  stopPollingOn?: string[];
}

/**
 * Hook for querying task status with automatic polling
 *
 * Polls task status every 2 seconds by default and stops when task completes.
 * Useful for tracking long-running tasks.
 *
 * @example
 * ```tsx
 * const { data: status, isLoading } = useTaskStatus(taskId, {
 *   pollingInterval: 1000,
 *   onSuccess: (data) => console.log('Status updated:', data)
 * });
 * ```
 */
export function useTaskStatus(
  taskId: string | null,
  options: UseTaskStatusOptions = {}
) {
  const {
    pollingInterval = 2000,
    stopPollingOn = ['completed', 'failed', 'cancelled', 'error'],
    ...queryOptions
  } = options;

  return useQuery({
    queryKey: ['task', taskId],
    queryFn: async () => {
      if (!taskId) throw new Error('Task ID is required');
      console.log('[useTaskStatus] Fetching status for task:', taskId);
      const response = await apiClient.getTaskStatus(taskId);
      console.log('[useTaskStatus] Status received:', response.status);
      return response;
    },
    enabled: !!taskId,
    refetchInterval: (query) => {
      const status = query.state.data?.status;
      // Stop polling if task reached terminal state
      if (status && stopPollingOn.includes(status)) {
        console.log('[useTaskStatus] Stopping polling, task status:', status);
        return false;
      }
      return pollingInterval;
    },
    retry: 3,
    retryDelay: (attemptIndex) => Math.min(1000 * Math.pow(2, attemptIndex), 5000),
    ...queryOptions,
  });
}

/**
 * Options for cancel task mutation
 */
interface UseCancelTaskOptions extends Omit<
  UseMutationOptions<void, Error, string, unknown>,
  'mutationFn'
> {}

/**
 * Hook for canceling a running task
 *
 * @example
 * ```tsx
 * const { mutate: cancelTask } = useCancelTask({
 *   onSuccess: () => console.log('Task canceled'),
 *   onError: (error) => console.error('Cancel failed:', error)
 * });
 * ```
 */
export function useCancelTask(options: UseCancelTaskOptions = {}) {
  return useMutation({
    mutationFn: async (taskId: string): Promise<void> => {
      console.log('[useCancelTask] Canceling task:', taskId);
      await apiClient.cancelTask(taskId);
      console.log('[useCancelTask] Task canceled successfully');
    },
    ...options,
  });
}
