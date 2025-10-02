// API client for ALEX backend server

import {
  CreateTaskRequest,
  CreateTaskResponse,
  TaskStatusResponse,
  SessionListResponse,
  SessionDetailsResponse,
} from './types';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

class APIError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    message: string
  ) {
    super(message);
    this.name = 'APIError';
  }
}

async function fetchAPI<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;

  try {
    const response = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    });

    if (!response.ok) {
      const errorText = await response.text();
      throw new APIError(
        response.status,
        response.statusText,
        errorText || `HTTP ${response.status}: ${response.statusText}`
      );
    }

    return await response.json();
  } catch (error) {
    if (error instanceof APIError) {
      throw error;
    }
    throw new Error(`Network error: ${error instanceof Error ? error.message : 'Unknown error'}`);
  }
}

// Task APIs

export async function createTask(
  request: CreateTaskRequest
): Promise<CreateTaskResponse> {
  return fetchAPI<CreateTaskResponse>('/api/tasks', {
    method: 'POST',
    body: JSON.stringify(request),
  });
}

export async function getTaskStatus(taskId: string): Promise<TaskStatusResponse> {
  return fetchAPI<TaskStatusResponse>(`/api/tasks/${taskId}`);
}

export async function cancelTask(taskId: string): Promise<void> {
  await fetchAPI(`/api/tasks/${taskId}/cancel`, {
    method: 'POST',
  });
}

// Session APIs

export async function listSessions(): Promise<SessionListResponse> {
  return fetchAPI<SessionListResponse>('/api/sessions');
}

export async function getSessionDetails(sessionId: string): Promise<SessionDetailsResponse> {
  return fetchAPI<SessionDetailsResponse>(`/api/sessions/${sessionId}`);
}

export async function deleteSession(sessionId: string): Promise<void> {
  await fetchAPI(`/api/sessions/${sessionId}`, {
    method: 'DELETE',
  });
}

export async function forkSession(sessionId: string): Promise<{ new_session_id: string }> {
  return fetchAPI<{ new_session_id: string }>(`/api/sessions/${sessionId}/fork`, {
    method: 'POST',
  });
}

// SSE Connection

export function createSSEConnection(sessionId: string): EventSource {
  const url = `${API_BASE_URL}/api/sse?session_id=${sessionId}`;
  return new EventSource(url);
}

// Health check

export async function healthCheck(): Promise<{ status: string }> {
  return fetchAPI<{ status: string }>('/health');
}

// Export API client object
export const apiClient = {
  createTask,
  getTaskStatus,
  cancelTask,
  listSessions,
  getSessionDetails,
  deleteSession,
  forkSession,
  createSSEConnection,
  healthCheck,
};
