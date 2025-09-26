import { z } from 'zod'
import { MessageSchema } from './message'

// Session state
export const SessionStateSchema = z.object({
  id: z.string(),
  status: z.enum(['active', 'paused', 'completed', 'error']),
  created_at: z.string(),
  updated_at: z.string(),
  message_count: z.number(),
  token_count: z.number(),
  compressed: z.boolean(),
  metadata: z.record(z.any()).optional(),
})

export type SessionState = z.infer<typeof SessionStateSchema>

// Session configuration
export const SessionConfigSchema = z.object({
  max_tokens: z.number().default(8000),
  compression_threshold: z.number().default(6000),
  auto_save: z.boolean().default(true),
  save_interval_ms: z.number().default(30000),
  tool_timeout_ms: z.number().default(60000),
  allowed_tools: z.array(z.string()).optional(),
  model_config: z.record(z.any()).optional(),
})

export type SessionConfig = z.infer<typeof SessionConfigSchema>

// Full session data
export const SessionSchema = z.object({
  state: SessionStateSchema,
  config: SessionConfigSchema,
  messages: z.array(MessageSchema),
  todos: z.array(z.record(z.any())).optional(),
})

export type Session = z.infer<typeof SessionSchema>

// Session management operations
export interface SessionManager {
  createSession(config?: Partial<SessionConfig>): Promise<Session>
  loadSession(id: string): Promise<Session>
  saveSession(session: Session): Promise<void>
  deleteSession(id: string): Promise<void>
  listSessions(): Promise<SessionState[]>
  resumeSession(id: string): Promise<Session>
}
