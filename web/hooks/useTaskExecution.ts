// Task execution hook using React Query

import { useMutation, useQuery } from '@tanstack/react-query';
import { apiClient } from '@/lib/api';
import { CreateTaskRequest, CreateTaskResponse, TaskStatusResponse } from '@/lib/types';

export function useTaskExecution() {
  return useMutation({
    mutationFn: async (request: CreateTaskRequest): Promise<CreateTaskResponse> => {
      return apiClient.createTask(request);
    },
    onError: (error: Error) => {
      console.error('Task execution failed:', error);
    },
  });
}

export function useTaskStatus(taskId: string | null) {
  return useQuery({
    queryKey: ['task', taskId],
    queryFn: () => {
      if (!taskId) throw new Error('Task ID is required');
      return apiClient.getTaskStatus(taskId);
    },
    enabled: !!taskId,
    refetchInterval: (query) => {
      // Stop polling if task is completed or failed
      if (query.state.data?.status === 'completed' || query.state.data?.status === 'failed') {
        return false;
      }
      return 2000; // Poll every 2 seconds
    },
  });
}

export function useCancelTask() {
  return useMutation({
    mutationFn: async (taskId: string): Promise<void> => {
      return apiClient.cancelTask(taskId);
    },
  });
}
