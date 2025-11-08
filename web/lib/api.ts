// API client for ALEX backend server

import {
  CreateTaskRequest,
  CreateTaskResponse,
  TaskStatusResponse,
  SessionListResponse,
  SessionDetailsResponse,
  ApprovePlanRequest,
  ApprovePlanResponse,
  CraftListResponse,
  CraftDownloadResponse,
  ArticleInsightResponse,
  ArticleCraftResponse,
  ArticleCraftListResponse,
  ImageConceptResponse,
  WebBlueprintResponse,
  CodePlanResponse,
  SaveArticleDraftPayload,
  GenerateImageConceptsPayload,
  GenerateWebBlueprintPayload,
  GenerateCodePlanPayload,
} from './types';
import { getAuthToken } from './auth';
import {
  getMockSessionList,
  getMockSessionDetails,
  deleteMockSession,
  forkMockSession,
  getMockCraftList,
  deleteMockCraft,
  getMockCraftDownloadUrl,
  buildMockArticleInsights,
  saveMockArticleDraft,
  getMockArticleDrafts,
  deleteMockArticleDraft,
  buildMockImageConcepts,
  buildMockWebBlueprint,
  buildMockCodePlan,
} from './mock-data';

const RAW_API_BASE_URL = process.env.NEXT_PUBLIC_API_URL?.trim();
const DEFAULT_INTERNAL_PRODUCTION_API_BASE = 'http://alex-server:8080';
const DEFAULT_DEVELOPMENT_API_BASE = 'http://localhost:8080';

function normalizeBaseUrl(url: string): string {
  return url.replace(/\/$/, '');
}

function resolveApiBaseUrl(): string {
  const value = RAW_API_BASE_URL;

  if (!value || value.toLowerCase() === 'auto') {
    if (typeof window !== 'undefined' && window.location?.origin) {
      return normalizeBaseUrl(window.location.origin);
    }

    return process.env.NODE_ENV === 'production'
      ? normalizeBaseUrl(DEFAULT_INTERNAL_PRODUCTION_API_BASE)
      : normalizeBaseUrl(DEFAULT_DEVELOPMENT_API_BASE);
  }

  return normalizeBaseUrl(value);
}

function buildApiUrl(endpoint: string): string {
  const baseUrl = resolveApiBaseUrl();

  if (!baseUrl) {
    return endpoint;
  }

  if (endpoint.startsWith('/')) {
    return `${baseUrl}${endpoint}`;
  }

  return `${baseUrl}/${endpoint}`;
}

const MOCK_FLAG = process.env.NEXT_PUBLIC_ENABLE_MOCK_DATA;
const ENABLE_MOCK_DATA =
  MOCK_FLAG === '1' ||
  (typeof MOCK_FLAG === 'string' && MOCK_FLAG.toLowerCase() === 'true') ||
  (MOCK_FLAG === undefined && process.env.NODE_ENV === 'development');

const loggedMockKeys = new Set<string>();

function logMockUsage(key: string, error?: unknown) {
  if (typeof console === 'undefined') return;
  const prefix = `[apiClient] Using mock data for ${key}`;
  if (!loggedMockKeys.has(key)) {
    loggedMockKeys.add(key);
    if (error) {
      console.warn(`${prefix} (fallback due to error)`, error);
    } else {
      console.info(prefix);
    }
    return;
  }

  if (error) {
    console.warn(`${prefix} (fallback due to error)`, error);
  }
}

function shouldFallbackToMock(error: unknown): boolean {
  if (error instanceof APIError) {
    return error.status >= 500;
  }
  return error instanceof Error && error.message.includes('Network error');
}

async function withMock<T>(
  key: string,
  fallback: () => T | Promise<T>,
  request?: () => Promise<T>
): Promise<T> {
  if (ENABLE_MOCK_DATA || !request) {
    logMockUsage(key);
    return await fallback();
  }

  try {
    return await request();
  } catch (error) {
    if (ENABLE_MOCK_DATA || shouldFallbackToMock(error)) {
      logMockUsage(key, error);
      return await fallback();
    }
    throw error;
  }
}

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
  const url = buildApiUrl(endpoint);

  try {
    const headers = new Headers(options?.headers || {});
    if (!headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }
    const token = getAuthToken();
    if (token) {
      headers.set('Authorization', `Bearer ${token}`);
    }

    const response = await fetch(url, {
      ...options,
      headers,
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
  return withMock('listSessions', getMockSessionList, () =>
    fetchAPI<SessionListResponse>('/api/sessions')
  );
}

export async function getSessionDetails(sessionId: string): Promise<SessionDetailsResponse> {
  return withMock('getSessionDetails', () => getMockSessionDetails(sessionId), () =>
    fetchAPI<SessionDetailsResponse>(`/api/sessions/${sessionId}`)
  );
}

export async function deleteSession(sessionId: string): Promise<void> {
  await withMock<void>(
    'deleteSession',
    () => {
      deleteMockSession(sessionId);
    },
    () =>
      fetchAPI(`/api/sessions/${sessionId}`, {
        method: 'DELETE',
      })
  );
}

export async function forkSession(sessionId: string): Promise<{ new_session_id: string }> {
  return withMock('forkSession', () => forkMockSession(sessionId), () =>
    fetchAPI<{ new_session_id: string }>(`/api/sessions/${sessionId}/fork`, {
      method: 'POST',
    })
  );
}

// SSE Connection

export function createSSEConnection(sessionId: string): EventSource {
  const token = getAuthToken();
  const origin =
    typeof window !== 'undefined' && window.location?.origin
      ? window.location.origin
      : 'http://localhost';
  const url = new URL(buildApiUrl('/api/sse'), origin);
  url.searchParams.set('session_id', sessionId);
  if (token) {
    url.searchParams.set('auth_token', token);
  }

  return new EventSource(url.toString());
}

// Health check

export async function healthCheck(): Promise<{ status: string }> {
  return fetchAPI<{ status: string }>('/health');
}

// Craft APIs

export async function listCrafts(): Promise<CraftListResponse> {
  return withMock('listCrafts', getMockCraftList, () =>
    fetchAPI<CraftListResponse>('/api/crafts')
  );
}

export async function deleteCraft(craftId: string): Promise<void> {
  await withMock<void>(
    'deleteCraft',
    () => {
      deleteMockCraft(craftId);
    },
    () =>
      fetchAPI(`/api/crafts/${craftId}`, {
        method: 'DELETE',
      })
  );
}

export async function getCraftDownloadUrl(
  craftId: string
): Promise<CraftDownloadResponse> {
  return withMock('getCraftDownloadUrl', () => getMockCraftDownloadUrl(craftId), () =>
    fetchAPI<CraftDownloadResponse>(`/api/crafts/${craftId}/download`)
  );
}

export async function generateArticleInsights(
  content: string
): Promise<ArticleInsightResponse> {
  return withMock('generateArticleInsights', () => buildMockArticleInsights(content), () =>
    fetchAPI<ArticleInsightResponse>('/api/workbench/article/insights', {
      method: 'POST',
      body: JSON.stringify({ content }),
    })
  );
}

export async function generateImageConcepts(
  payload: GenerateImageConceptsPayload
): Promise<ImageConceptResponse> {
  return withMock('generateImageConcepts', () => buildMockImageConcepts(payload), () =>
    fetchAPI<ImageConceptResponse>('/api/workbench/image/concepts', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  );
}

export async function generateWebBlueprint(
  payload: GenerateWebBlueprintPayload
): Promise<WebBlueprintResponse> {
  return withMock('generateWebBlueprint', () => buildMockWebBlueprint(payload), () =>
    fetchAPI<WebBlueprintResponse>('/api/workbench/web/blueprint', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  );
}

export async function generateCodePlan(
  payload: GenerateCodePlanPayload
): Promise<CodePlanResponse> {
  return withMock('generateCodePlan', () => buildMockCodePlan(payload), () =>
    fetchAPI<CodePlanResponse>('/api/workbench/code/plan', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  );
}

export async function saveArticleDraft(
  payload: SaveArticleDraftPayload
): Promise<ArticleCraftResponse> {
  return withMock('saveArticleDraft', () => saveMockArticleDraft(payload), () =>
    fetchAPI<ArticleCraftResponse>('/api/workbench/article/crafts', {
      method: 'POST',
      body: JSON.stringify(payload),
    })
  );
}

export async function listArticleDrafts(): Promise<ArticleCraftListResponse> {
  return withMock('listArticleDrafts', getMockArticleDrafts, () =>
    fetchAPI<ArticleCraftListResponse>('/api/workbench/article/crafts')
  );
}

export async function deleteArticleDraft(craftId: string): Promise<void> {
  await withMock<void>(
    'deleteArticleDraft',
    () => {
      deleteMockArticleDraft(craftId);
    },
    () =>
      fetchAPI(`/api/workbench/article/crafts/${craftId}`, {
        method: 'DELETE',
      })
  );
}

// Export API client object
export type {
  GenerateImageConceptsPayload,
  GenerateWebBlueprintPayload,
  GenerateCodePlanPayload,
  SaveArticleDraftPayload,
};

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
  listCrafts,
  deleteCraft,
  getCraftDownloadUrl,
  generateArticleInsights,
  generateImageConcepts,
  generateWebBlueprint,
  generateCodePlan,
  saveArticleDraft,
  listArticleDrafts,
  deleteArticleDraft,
};

