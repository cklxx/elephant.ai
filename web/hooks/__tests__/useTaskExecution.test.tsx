import { describe, it, expect, beforeEach, vi } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { useTaskExecution, useTaskStatus, useCancelTask } from '../useTaskExecution';
import { apiClient } from '@/lib/api';
import { ReactNode } from 'react';

// Mock the API client
vi.mock('@/lib/api', () => ({
  apiClient: {
    createTask: vi.fn(),
    getTaskStatus: vi.fn(),
    cancelTask: vi.fn(),
  },
}));

// Create a wrapper with QueryClient
function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  Wrapper.displayName = "QueryClientWrapper";
  return Wrapper;
}

describe('useTaskExecution', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.removeItem('alex-llm-selection');
  });

  describe('Task Creation', () => {
    it('should create a task successfully', async () => {
      const mockResponse = {
        run_id: 'task-123',
        session_id: 'session-456',
        status: 'pending' as const,
      };

      vi.mocked(apiClient.createTask).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useTaskExecution(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ task: 'Build a web scraper' });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(apiClient.createTask).toHaveBeenCalledWith({
        task: 'Build a web scraper',
      });
      expect(result.current.data).toEqual(mockResponse);
    });

    it('should handle task creation errors', async () => {
      const mockError = new Error('Failed to create task');
      vi.mocked(apiClient.createTask).mockRejectedValue(mockError);

      const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {});

      const { result } = renderHook(() => useTaskExecution(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ task: 'Invalid task' });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(consoleErrorSpy).toHaveBeenCalledWith(
        '[useTaskExecution] Task execution failed:',
        mockError
      );
      expect(result.current.error).toBe(mockError);

      consoleErrorSpy.mockRestore();
    });

    it('should accept session_id in request', async () => {
      const mockResponse = {
        run_id: 'task-123',
        session_id: 'existing-session',
        status: 'pending' as const,
      };

      vi.mocked(apiClient.createTask).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useTaskExecution(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({
        task: 'Continue previous work',
        session_id: 'existing-session',
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(apiClient.createTask).toHaveBeenCalledWith({
        task: 'Continue previous work',
        session_id: 'existing-session',
      });
    });

    it('includes llm_selection from localStorage', async () => {
      const mockResponse = {
        run_id: 'task-123',
        session_id: 'session-456',
        status: 'pending' as const,
      };

      localStorage.setItem('alex-llm-selection', JSON.stringify({
        mode: 'cli',
        provider: 'codex',
        model: 'gpt-5.2-codex',
        source: 'codex_cli',
      }));

      vi.mocked(apiClient.createTask).mockResolvedValue(mockResponse);

      const { result } = renderHook(() => useTaskExecution(), {
        wrapper: createWrapper(),
      });

      result.current.mutate({ task: 'hello' });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(apiClient.createTask).toHaveBeenCalledWith({
        task: 'hello',
        llm_selection: {
          mode: 'cli',
          provider: 'codex',
          model: 'gpt-5.2-codex',
          source: 'codex_cli',
        },
      });
    });
  });
});

describe('useTaskStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should not query when taskId is null', () => {
    renderHook(() => useTaskStatus(null), {
      wrapper: createWrapper(),
    });

    expect(apiClient.getTaskStatus).not.toHaveBeenCalled();
  });

  it('should fetch task status when taskId is provided', async () => {
    const mockStatus = {
      run_id: 'task-123',
      session_id: 'session-456',
      status: 'running' as const,
      created_at: new Date().toISOString(),
    };

    vi.mocked(apiClient.getTaskStatus).mockResolvedValue(mockStatus);

    const { result } = renderHook(() => useTaskStatus('task-123'), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(apiClient.getTaskStatus).toHaveBeenCalledWith('task-123');
    expect(result.current.data).toEqual(mockStatus);
  });

  it('should poll every 2 seconds when task is running', async () => {
    const mockRunningStatus = {
      run_id: 'task-123',
      session_id: 'session-456',
      status: 'running' as const,
      created_at: new Date().toISOString(),
    };

    vi.mocked(apiClient.getTaskStatus).mockResolvedValue(mockRunningStatus);

    const pollingInterval = 50;
    renderHook(() => useTaskStatus('task-123', { pollingInterval }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(apiClient.getTaskStatus).toHaveBeenCalled();
    });

    const initialCallCount = vi.mocked(apiClient.getTaskStatus).mock.calls.length;

    await new Promise((resolve) => setTimeout(resolve, pollingInterval * 2));

    expect(vi.mocked(apiClient.getTaskStatus).mock.calls.length).toBeGreaterThan(initialCallCount);
  });

  it('should stop polling when task is completed', async () => {
    const mockCompletedStatus = {
      run_id: 'task-123',
      session_id: 'session-456',
      status: 'completed' as const,
      created_at: new Date().toISOString(),
      completed_at: new Date().toISOString(),
    };

    vi.mocked(apiClient.getTaskStatus).mockResolvedValue(mockCompletedStatus);

    const pollingInterval = 50;
    renderHook(() => useTaskStatus('task-123', { pollingInterval }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(apiClient.getTaskStatus).toHaveBeenCalled();
    });

    const callCountAfterComplete = vi.mocked(apiClient.getTaskStatus).mock.calls.length;

    await new Promise((resolve) => setTimeout(resolve, pollingInterval * 4));

    expect(vi.mocked(apiClient.getTaskStatus).mock.calls.length).toBe(callCountAfterComplete);
  });

  it('should stop polling when task fails', async () => {
    const mockFailedStatus = {
      run_id: 'task-123',
      session_id: 'session-456',
      status: 'failed' as const,
      created_at: new Date().toISOString(),
      error: 'Task failed due to error',
    };

    vi.mocked(apiClient.getTaskStatus).mockResolvedValue(mockFailedStatus);

    const pollingInterval = 50;
    renderHook(() => useTaskStatus('task-123', { pollingInterval }), {
      wrapper: createWrapper(),
    });

    await waitFor(() => {
      expect(apiClient.getTaskStatus).toHaveBeenCalled();
    });

    const callCountAfterFail = vi.mocked(apiClient.getTaskStatus).mock.calls.length;

    await new Promise((resolve) => setTimeout(resolve, pollingInterval * 4));

    expect(vi.mocked(apiClient.getTaskStatus).mock.calls.length).toBe(callCountAfterFail);
  });
});

describe('useCancelTask', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should cancel task successfully', async () => {
    vi.mocked(apiClient.cancelTask).mockResolvedValue(undefined);

    const { result } = renderHook(() => useCancelTask(), {
      wrapper: createWrapper(),
    });

    result.current.mutate('task-123');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(apiClient.cancelTask).toHaveBeenCalledWith('task-123');
  });

  it('should handle cancel errors', async () => {
    const mockError = new Error('Failed to cancel task');
    vi.mocked(apiClient.cancelTask).mockRejectedValue(mockError);

    const { result } = renderHook(() => useCancelTask(), {
      wrapper: createWrapper(),
    });

    result.current.mutate('task-123');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBe(mockError);
  });
});
