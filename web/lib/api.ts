// API client for ALEX backend server

import {
  CreateTaskRequest,
  CreateTaskResponse,
  TaskStatusResponse,
  SessionListResponse,
  SessionDetailsResponse,
  ApprovePlanRequest,
  ApprovePlanResponse,
} from './types';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080';

export class APIError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    message: string,
    public details?: string,
    public payload?: unknown,
    public rawBody?: string
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
      const contentType = response.headers.get('content-type') ?? '';

      let parsedBody: unknown;
      if (errorText && contentType.includes('application/json')) {
        try {
          parsedBody = JSON.parse(errorText);
        } catch (parseError) {
          console.warn(
            '[apiClient] Failed to parse error JSON response',
            parseError
          );
        }
      }

      const defaultMessage = `HTTP ${response.status}: ${response.statusText || 'Unknown Status'}`;
      let message = defaultMessage;
      let details: string | undefined;

      if (parsedBody && typeof parsedBody === 'object' && !Array.isArray(parsedBody)) {
        const body = parsedBody as Record<string, unknown>;
        const errorMessage = body.error;
        const errorDetails = body.details ?? body.message;

        if (typeof errorMessage === 'string' && errorMessage.trim().length > 0) {
          message = errorMessage.trim();
        }

        if (typeof errorDetails === 'string' && errorDetails.trim().length > 0) {
          const trimmedDetails = errorDetails.trim();
          details = trimmedDetails !== message ? trimmedDetails : undefined;
        }
      } else if (errorText.trim().length > 0) {
        message = errorText.trim();
      }

      throw new APIError(
        response.status,
        response.statusText,
        message,
        details,
        parsedBody,
        errorText || undefined
      );
    }

    // Handle 204 No Content and other responses without body
    if (response.status === 204 || response.headers.get('content-length') === '0') {
      return undefined as T;
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

export async function approvePlan(
  request: ApprovePlanRequest
): Promise<ApprovePlanResponse> {
  return fetchAPI<ApprovePlanResponse>('/api/plans/approve', {
    method: 'POST',
    body: JSON.stringify(request),
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
  approvePlan,
  listSessions,
  getSessionDetails,
  deleteSession,
  forkSession,
  createSSEConnection,
  healthCheck,
};

