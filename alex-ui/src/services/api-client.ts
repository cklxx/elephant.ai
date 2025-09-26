import {
  ApiClient,
  ApiRequest,
  ChatRequest,
  ChatResponse,
  CreateSessionRequest,
  LoadSessionResponse,
  Session,
  SessionState,
  ToolDefinition,
} from '@/types'

class ApiClientImpl implements ApiClient {
  private baseUrl: string
  private defaultHeaders: Record<string, string>

  constructor(baseUrl = 'http://localhost:8080/api/v1') {
    this.baseUrl = baseUrl
    this.defaultHeaders = {
      'Content-Type': 'application/json',
    }
  }

  private async request<T>(request: ApiRequest): Promise<T> {
    const url = `${this.baseUrl}${request.endpoint}`
    const headers = { ...this.defaultHeaders, ...request.headers }

    try {
      const response = await fetch(url, {
        method: request.method,
        headers,
        body: request.body ? JSON.stringify(request.body) : undefined,
        signal: request.timeout
          ? AbortSignal.timeout(request.timeout)
          : undefined,
      })

      if (!response.ok) {
        throw new Error(`HTTP ${response.status}: ${response.statusText}`)
      }

      const data = await response.json()
      return data
    } catch (error) {
      if (error instanceof Error) {
        throw new Error(`API request failed: ${error.message}`)
      }
      throw error
    }
  }

  async createSession(request: CreateSessionRequest): Promise<Session> {
    return this.request<Session>({
      method: 'POST',
      endpoint: '/sessions',
      body: request,
    })
  }

  async loadSession(id: string): Promise<LoadSessionResponse> {
    return this.request<LoadSessionResponse>({
      method: 'GET',
      endpoint: `/sessions/${id}`,
    })
  }

  async deleteSession(id: string): Promise<void> {
    await this.request<void>({
      method: 'DELETE',
      endpoint: `/sessions/${id}`,
    })
  }

  async listSessions(): Promise<SessionState[]> {
    return this.request<SessionState[]>({
      method: 'GET',
      endpoint: '/sessions',
    })
  }

  async sendMessage(request: ChatRequest): Promise<ChatResponse> {
    return this.request<ChatResponse>({
      method: 'POST',
      endpoint: '/chat',
      body: request,
    })
  }

  async getAvailableTools(): Promise<ToolDefinition[]> {
    return this.request<ToolDefinition[]>({
      method: 'GET',
      endpoint: '/tools',
    })
  }

  async health(): Promise<{ status: string; version: string }> {
    return this.request<{ status: string; version: string }>({
      method: 'GET',
      endpoint: '/health',
    })
  }
}

export const apiClient = new ApiClientImpl()
