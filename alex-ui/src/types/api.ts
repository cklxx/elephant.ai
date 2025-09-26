import { z } from 'zod'
import { MessageSchema, StreamingChunkSchema, StreamingChunk } from './message'
import {
  SessionSchema,
  SessionStateSchema,
  Session,
  SessionState,
} from './session'
import { ToolDefinitionSchema, ToolDefinition } from './tool'

// API Request/Response types
export const ApiRequestSchema = z.object({
  method: z.enum(['GET', 'POST', 'PUT', 'DELETE', 'PATCH']),
  endpoint: z.string(),
  headers: z.record(z.string()).optional(),
  body: z.any().optional(),
  timeout: z.number().optional(),
})

export type ApiRequest = z.infer<typeof ApiRequestSchema>

export const ApiResponseSchema = z.object({
  status: z.number(),
  statusText: z.string(),
  headers: z.record(z.string()),
  data: z.any(),
  error: z.string().optional(),
})

export type ApiResponse = z.infer<typeof ApiResponseSchema>

// Chat API endpoints
export const ChatRequestSchema = z.object({
  message: z.string(),
  session_id: z.string().optional(),
  stream: z.boolean().default(true),
  model_config: z.record(z.any()).optional(),
  tool_config: z.record(z.any()).optional(),
})

export type ChatRequest = z.infer<typeof ChatRequestSchema>

export const ChatResponseSchema = z.object({
  message: MessageSchema,
  session_id: z.string(),
  token_usage: z
    .object({
      prompt_tokens: z.number(),
      completion_tokens: z.number(),
      total_tokens: z.number(),
    })
    .optional(),
})

export type ChatResponse = z.infer<typeof ChatResponseSchema>

// Session API endpoints
export const CreateSessionRequestSchema = z.object({
  config: z.record(z.any()).optional(),
  initial_message: z.string().optional(),
})

export type CreateSessionRequest = z.infer<typeof CreateSessionRequestSchema>

export const LoadSessionResponseSchema = z.object({
  session: SessionSchema,
  available_tools: z.array(ToolDefinitionSchema),
})

export type LoadSessionResponse = z.infer<typeof LoadSessionResponseSchema>

// WebSocket event types
export const WebSocketEventSchema = z.discriminatedUnion('type', [
  z.object({
    type: z.literal('connect'),
    session_id: z.string(),
  }),
  z.object({
    type: z.literal('disconnect'),
    reason: z.string().optional(),
  }),
  z.object({
    type: z.literal('message'),
    data: ChatRequestSchema,
  }),
  z.object({
    type: z.literal('stream_chunk'),
    data: StreamingChunkSchema,
  }),
  z.object({
    type: z.literal('error'),
    error: z.string(),
    code: z.string().optional(),
  }),
  z.object({
    type: z.literal('session_update'),
    session_state: SessionStateSchema,
  }),
])

export type WebSocketEvent = z.infer<typeof WebSocketEventSchema>

// API client interface
export interface ApiClient {
  // Session management
  createSession(request: CreateSessionRequest): Promise<Session>
  loadSession(id: string): Promise<LoadSessionResponse>
  deleteSession(id: string): Promise<void>
  listSessions(): Promise<SessionState[]>

  // Chat
  sendMessage(request: ChatRequest): Promise<ChatResponse>

  // Tools
  getAvailableTools(): Promise<ToolDefinition[]>

  // Health check
  health(): Promise<{ status: string; version: string }>
}

// WebSocket client interface
export interface WebSocketClient {
  connect(url: string): Promise<void>
  disconnect(): void
  send(event: WebSocketEvent): void
  onMessage(callback: (chunk: StreamingChunk) => void): void
  onError(callback: (error: string) => void): void
  onConnect(callback: () => void): void
  onDisconnect(callback: (reason?: string) => void): void
  isConnected(): boolean
}
