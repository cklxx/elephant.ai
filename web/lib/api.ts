// API client for ALEX backend server

import { buildApiUrl } from "./api-base";
import { authClient } from "./auth/client";
import { createLogger } from "./logger";
import {
  CreateTaskRequest,
  CreateTaskResponse,
  TaskStatusResponse,
  SessionListResponse,
  SessionDetailsResponse,
  ShareTokenResponse,
  SharedSessionResponse,
  AppsConfigSnapshot,
  AppsConfigUpdatePayload,
  RuntimeConfigSnapshot,
  RuntimeConfigOverridesPayload,
  EvaluationListResponse,
  EvaluationDetailResponse,
  StartEvaluationRequest,
  EvaluationJobSummary,
  ContextWindowPreviewResponse,
  ContextConfigSnapshot,
  ContextConfigUpdatePayload,
  SandboxBrowserInfo,
  UserPersonaProfile,
  RuntimeModelCatalog,
  LogTraceBundle,
} from "./types";

export interface ApiRequestOptions extends RequestInit {
  skipAuth?: boolean;
}

export class APIError extends Error {
  constructor(
    public status: number,
    public statusText: string,
    message: string,
    public details?: string,
    public payload?: unknown,
    public rawBody?: string,
  ) {
    super(message);
    this.name = "APIError";
  }
}

const log = createLogger("API");

async function fetchAPI<T>(
  endpoint: string,
  options: ApiRequestOptions = {},
  attempt = 0,
): Promise<T> {
  const url = buildApiUrl(endpoint);
  const { skipAuth = false, headers: inputHeaders, ...rest } = options;

  const headers = new Headers(inputHeaders ?? {});

  if (rest.body !== undefined && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }

  if (!skipAuth) {
    const token = await authClient.ensureAccessToken();
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
  }

  try {
    const response = await fetch(url, {
      ...rest,
      headers,
      credentials: rest.credentials ?? "include",
    });

    if (response.status === 401 && !skipAuth && attempt < 1) {
      try {
        await authClient.refresh();
      } catch (refreshError) {
        log.warn("Refresh token failed after 401", { error: refreshError });
      }
      return fetchAPI<T>(endpoint, options, attempt + 1);
    }

    if (!response.ok) {
      const errorText = await response.text();
      const contentType = response.headers.get("content-type") ?? "";

      let parsedBody: unknown;
      if (errorText && contentType.includes("application/json")) {
        try {
          parsedBody = JSON.parse(errorText);
        } catch (parseError) {
          log.warn("Failed to parse error JSON response", { error: parseError });
        }
      }

      const defaultMessage = `HTTP ${response.status}: ${response.statusText || "Unknown Status"}`;
      let message = defaultMessage;
      let details: string | undefined;

      if (
        parsedBody &&
        typeof parsedBody === "object" &&
        !Array.isArray(parsedBody)
      ) {
        const body = parsedBody as Record<string, unknown>;
        const errorMessage = body.error;
        const errorDetails = body.details ?? body.message;

        if (
          typeof errorMessage === "string" &&
          errorMessage.trim().length > 0
        ) {
          message = errorMessage.trim();
        }

        if (
          typeof errorDetails === "string" &&
          errorDetails.trim().length > 0
        ) {
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
        errorText || undefined,
      );
    }

    if (
      response.status === 204 ||
      response.headers.get("content-length") === "0"
    ) {
      return undefined as T;
    }

    return await response.json();
  } catch (error) {
    if (error instanceof APIError) {
      throw error;
    }
    throw new Error(
      `Network error: ${error instanceof Error ? error.message : "Unknown error"}`,
    );
  }
}

// Task APIs

export async function createTask(
  request: CreateTaskRequest,
): Promise<CreateTaskResponse> {
  return fetchAPI<CreateTaskResponse>("/api/tasks", {
    method: "POST",
    body: JSON.stringify(request),
  });
}

export async function getTaskStatus(
  taskId: string,
): Promise<TaskStatusResponse> {
  return fetchAPI<TaskStatusResponse>(`/api/tasks/${taskId}`);
}

export async function cancelTask(taskId: string): Promise<void> {
  await fetchAPI(`/api/tasks/${taskId}/cancel`, {
    method: "POST",
  });
}

// Internal runtime config APIs

export async function getRuntimeConfigSnapshot(): Promise<RuntimeConfigSnapshot> {
  return fetchAPI<RuntimeConfigSnapshot>("/api/internal/config/runtime");
}

export async function getAppsConfigSnapshot(): Promise<AppsConfigSnapshot> {
  return fetchAPI<AppsConfigSnapshot>("/api/internal/config/apps");
}

export async function getSandboxBrowserInfo(
  sessionId?: string | null,
): Promise<SandboxBrowserInfo> {
  const query = sessionId ? `?session_id=${encodeURIComponent(sessionId)}` : "";
  return fetchAPI<SandboxBrowserInfo>(`/api/sandbox/browser-info${query}`);
}

export async function updateRuntimeConfig(
  request: RuntimeConfigOverridesPayload,
): Promise<RuntimeConfigSnapshot> {
  return fetchAPI<RuntimeConfigSnapshot>("/api/internal/config/runtime", {
    method: "PUT",
    body: JSON.stringify(request),
  });
}

export async function updateAppsConfig(
  request: AppsConfigUpdatePayload,
): Promise<AppsConfigSnapshot> {
  return fetchAPI<AppsConfigSnapshot>("/api/internal/config/apps", {
    method: "PUT",
    body: JSON.stringify(request),
  });
}

export async function getRuntimeModelCatalog(): Promise<RuntimeModelCatalog> {
  return fetchAPI<RuntimeModelCatalog>("/api/internal/config/runtime/models");
}

export async function getSubscriptionCatalog(): Promise<RuntimeModelCatalog> {
  return fetchAPI<RuntimeModelCatalog>("/api/internal/subscription/catalog");
}

// Dev context config APIs

export async function getContextConfig(): Promise<ContextConfigSnapshot> {
  return fetchAPI<ContextConfigSnapshot>("/api/dev/context-config");
}

export async function updateContextConfig(
  request: ContextConfigUpdatePayload,
): Promise<ContextConfigSnapshot> {
  return fetchAPI<ContextConfigSnapshot>("/api/dev/context-config", {
    method: "PUT",
    body: JSON.stringify(request),
  });
}

export async function getContextConfigPreview(params?: {
  personaKey?: string;
  goalKey?: string;
  worldKey?: string;
  toolMode?: string;
  toolPreset?: string;
}): Promise<ContextWindowPreviewResponse> {
  const search = new URLSearchParams();
  if (params?.personaKey) search.set("persona_key", params.personaKey);
  if (params?.goalKey) search.set("goal_key", params.goalKey);
  if (params?.worldKey) search.set("world_key", params.worldKey);
  if (params?.toolMode) search.set("tool_mode", params.toolMode);
  if (params?.toolPreset) search.set("tool_preset", params.toolPreset);
  const suffix = search.toString();
  const endpoint = suffix ? `/api/dev/context-config/preview?${suffix}` : "/api/dev/context-config/preview";
  return fetchAPI<ContextWindowPreviewResponse>(endpoint);
}

// Session APIs

export async function listSessions(): Promise<SessionListResponse> {
  return fetchAPI<SessionListResponse>("/api/sessions");
}

export async function getSessionDetails(
  sessionId: string,
): Promise<SessionDetailsResponse> {
  type SessionRecord = {
    id: string;
    created_at: string;
    updated_at: string;
  };

  type TaskListResponse = {
    tasks: TaskStatusResponse[];
    total?: number;
    limit?: number;
    offset?: number;
  };

  const session = await fetchAPI<SessionRecord>(`/api/sessions/${sessionId}`);

  let tasks: TaskStatusResponse[] = [];
  try {
    const params = new URLSearchParams({
      session_id: sessionId,
      limit: "100",
      offset: "0",
    });
    const response = await fetchAPI<TaskListResponse>(`/api/tasks?${params.toString()}`);
    const rawTasks = Array.isArray(response?.tasks) ? response.tasks : [];
    tasks = rawTasks.filter((task) => !task.session_id || task.session_id === sessionId);
  } catch (error) {
    log.warn("Failed to load session tasks", {
      sessionId,
      error: error instanceof Error ? error.message : error,
    });
  }

  const now = new Date().toISOString();
  const taskSummaries = tasks.map((task) => ({
    run_id: task.run_id,
    parent_run_id: task.parent_run_id ?? null,
    status: task.status,
    created_at: task.created_at ?? now,
    updated_at: task.completed_at ?? task.updated_at ?? undefined,
    final_answer: task.final_answer ?? null,
  }));

  return {
    session: {
      id: session.id,
      created_at: session.created_at,
      updated_at: session.updated_at,
      task_count: taskSummaries.length,
      last_task: null,
    },
    tasks: taskSummaries,
  };
}

export async function getSessionRaw(sessionId: string): Promise<{
  session: Record<string, unknown>;
  tasks: Record<string, unknown>;
}> {
  const session = await fetchAPI<Record<string, unknown>>(
    `/api/sessions/${sessionId}`,
  );
  const params = new URLSearchParams({
    session_id: sessionId,
    limit: "100",
    offset: "0",
  });
  const tasks = await fetchAPI<Record<string, unknown>>(
    `/api/tasks?${params.toString()}`,
  );
  return { session, tasks };
}

export type SessionSnapshotsResponse = {
  session_id: string;
  items: Array<{
    turn_id: number;
    llm_turn_seq: number;
    summary: string;
    created_at: string;
  }>;
  next_cursor?: string;
};

export async function listSessionSnapshots(
  sessionId: string,
  limit = 20,
  cursor = "",
): Promise<SessionSnapshotsResponse> {
  const params = new URLSearchParams({
    limit: String(limit),
  });
  if (cursor) {
    params.set("cursor", cursor);
  }
  const encoded = encodeURIComponent(sessionId);
  return fetchAPI<SessionSnapshotsResponse>(
    `/api/sessions/${encoded}/snapshots?${params.toString()}`,
  );
}

export async function getSessionTurnSnapshot(
  sessionId: string,
  turnId: number,
): Promise<Record<string, unknown>> {
  const encoded = encodeURIComponent(sessionId);
  return fetchAPI<Record<string, unknown>>(
    `/api/sessions/${encoded}/turns/${turnId}`,
  );
}

export async function getSessionTitle(
  sessionId: string,
): Promise<string | null> {
  type SessionRecord = {
    id: string;
    metadata?: Record<string, string>;
  };

  const session = await fetchAPI<SessionRecord>(`/api/sessions/${sessionId}`);
  const title = session?.metadata?.title;
  if (typeof title !== "string") {
    return null;
  }
  const trimmed = title.trim();
  return trimmed ? trimmed : null;
}

export async function getSessionPersona(
  sessionId: string,
): Promise<UserPersonaProfile | null> {
  type SessionPersonaResponse = {
    session_id: string;
    user_persona?: UserPersonaProfile | null;
  };
  const sanitized = encodeURIComponent(sessionId);
  const response = await fetchAPI<SessionPersonaResponse>(
    `/api/sessions/${sanitized}/persona`,
  );
  return response.user_persona ?? null;
}

export async function updateSessionPersona(
  sessionId: string,
  persona: UserPersonaProfile,
): Promise<UserPersonaProfile> {
  type SessionPersonaResponse = {
    session_id: string;
    user_persona: UserPersonaProfile;
  };
  const sanitized = encodeURIComponent(sessionId);
  const response = await fetchAPI<SessionPersonaResponse>(
    `/api/sessions/${sanitized}/persona`,
    {
      method: "PUT",
      body: JSON.stringify({ user_persona: persona }),
    },
  );
  return response.user_persona;
}

export async function deleteSession(sessionId: string): Promise<void> {
  await fetchAPI(`/api/sessions/${sessionId}`, {
    method: "DELETE",
  });
}

export async function createSessionShare(
  sessionId: string,
): Promise<ShareTokenResponse> {
  return fetchAPI<ShareTokenResponse>(`/api/sessions/${sessionId}/share`, {
    method: "POST",
  });
}

export async function getSharedSession(
  sessionId: string,
  token: string,
): Promise<SharedSessionResponse> {
  const params = new URLSearchParams({ token });
  return fetchAPI<SharedSessionResponse>(
    `/api/share/sessions/${sessionId}?${params.toString()}`,
    { skipAuth: true },
  );
}

export async function getContextWindowPreview(
  sessionId: string,
): Promise<ContextWindowPreviewResponse> {
  const sanitized = encodeURIComponent(sessionId);
  return fetchAPI<ContextWindowPreviewResponse>(`/api/dev/sessions/${sanitized}/context-window`);
}

export async function forkSession(
  sessionId: string,
): Promise<{ new_session_id: string }> {
  return fetchAPI<{ new_session_id: string }>(
    `/api/sessions/${sessionId}/fork`,
    {
      method: "POST",
    },
  );
}

// Evaluation APIs
export async function listEvaluations(): Promise<EvaluationListResponse> {
  return fetchAPI<EvaluationListResponse>("/api/evaluations");
}

export async function startEvaluation(
  request: StartEvaluationRequest,
): Promise<EvaluationJobSummary> {
  return fetchAPI<EvaluationJobSummary>("/api/evaluations", {
    method: "POST",
    body: JSON.stringify(request),
  });
}

export async function getEvaluation(
  evaluationId: string,
): Promise<EvaluationDetailResponse> {
  return fetchAPI<EvaluationDetailResponse>(`/api/evaluations/${evaluationId}`);
}

// Sessions

export async function createSession(): Promise<{ session_id: string }> {
  return fetchAPI<{ session_id: string }>("/api/sessions", {
    method: "POST",
  });
}

// Dev log trace
export async function getLogTrace(logId: string): Promise<LogTraceBundle> {
  return fetchAPI<LogTraceBundle>(
    `/api/dev/logs?log_id=${encodeURIComponent(logId)}`,
  );
}

// Dev memory query

export type MemoryDailyEntry = {
  date: string;
  path: string;
  content: string;
};

export type MemorySnapshot = {
  user_id: string;
  long_term: string;
  daily: MemoryDailyEntry[];
};

export async function getMemorySnapshot(sessionId: string): Promise<MemorySnapshot> {
  const params = new URLSearchParams({ session_id: sessionId });
  return fetchAPI<MemorySnapshot>(`/api/dev/memory?${params.toString()}`);
}

// SSE Connection

export type SSEReplayMode = "full" | "session" | "none";

export function sseRequiresAccessToken(): boolean {
  if (typeof window === "undefined") {
    return true;
  }
  const url = new URL(buildApiUrl("/api/sse"));
  return !window.location?.origin || url.origin !== window.location.origin;
}

export function createSSEConnection(
  sessionId: string,
  accessToken?: string,
  options: { replay?: SSEReplayMode; debug?: boolean } = {},
): EventSource {
  const url = new URL(buildApiUrl("/api/sse"));
  url.searchParams.set("session_id", sessionId);

  const replay = options.replay ?? "full";
  if (replay !== "full") {
    url.searchParams.set("replay", replay);
  }
  if (options.debug) {
    url.searchParams.set("debug", "1");
  }

  const shouldIncludeAccessToken = sseRequiresAccessToken();

  if (shouldIncludeAccessToken) {
    const token = accessToken ?? authClient.getSession()?.accessToken;
    if (token) {
      url.searchParams.set("access_token", token);
    }
  }
  return new EventSource(url.toString(), { withCredentials: true });
}

// Health check

export async function healthCheck(): Promise<{ status: string }> {
  return fetchAPI<{ status: string }>("/health", { skipAuth: true });
}

// Export API client object
export const apiClient = {
  createTask,
  getTaskStatus,
  cancelTask,
  createSession,
  listSessions,
  getSessionDetails,
  getSessionRaw,
  getSessionTitle,
  listSessionSnapshots,
  getSessionTurnSnapshot,
  deleteSession,
  getLogTrace,
  getMemorySnapshot,
  createSessionShare,
  getSharedSession,
  getContextWindowPreview,
  getContextConfig,
  updateContextConfig,
  getContextConfigPreview,
  forkSession,
  listEvaluations,
  startEvaluation,
  getEvaluation,
  createSSEConnection,
  healthCheck,
};
