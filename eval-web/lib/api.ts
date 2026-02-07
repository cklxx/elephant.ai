const BASE_URL =
  typeof window !== "undefined"
    ? process.env.NEXT_PUBLIC_EVAL_API_URL ?? "http://localhost:8081"
    : "http://localhost:8081";

export interface ApiError {
  error: string;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const url = `${BASE_URL}${path}`;
  const res = await fetch(url, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init?.headers,
    },
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error ?? `HTTP ${res.status}`);
  }
  return res.json();
}

export const api = {
  // Health
  health: () => request<{ status: string }>("/health"),

  // Evaluations
  listEvaluations: () =>
    request<{ evaluations: any[] }>("/api/evaluations"),
  getEvaluation: (id: string) =>
    request<{ evaluation: any; results: any }>(`/api/evaluations/${id}`),
  deleteEvaluation: (id: string) =>
    request<{ deleted: string }>(`/api/evaluations/${id}`, { method: "DELETE" }),

  // Agents
  listAgents: () => request<{ agents: any[] }>("/api/agents"),
  getAgent: (id: string) => request<any>(`/api/agents/${id}`),
  getAgentEvaluations: (id: string) =>
    request<{ evaluations: any[] }>(`/api/agents/${id}/evaluations`),

  // RL Data (stubs for Batch 3)
  getRLStats: () => request<any>("/api/rl/stats"),
  listTrajectories: (params?: Record<string, string>) => {
    const qs = params ? "?" + new URLSearchParams(params).toString() : "";
    return request<any>(`/api/rl/trajectories${qs}`);
  },

  // Eval Tasks (stubs for Batch 4)
  listEvalTasks: () => request<{ tasks: any[] }>("/api/eval-tasks"),
  getEvalTask: (id: string) => request<any>(`/api/eval-tasks/${id}`),
};
