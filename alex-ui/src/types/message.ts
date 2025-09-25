import { z } from 'zod'

// Message types matching Go backend
export const MessageRoleSchema = z.enum(['user', 'assistant', 'system', 'tool'])
export type MessageRole = z.infer<typeof MessageRoleSchema>

export const MessageContentSchema = z.object({
  type: z.enum(['text', 'tool_call', 'tool_result']),
  text: z.string().optional(),
  tool_call_id: z.string().optional(),
  tool_name: z.string().optional(),
  tool_input: z.record(z.any()).optional(),
  tool_result: z.any().optional(),
})

export type MessageContent = z.infer<typeof MessageContentSchema>

export const MessageSchema = z.object({
  id: z.string(),
  role: MessageRoleSchema,
  content: z.array(MessageContentSchema),
  timestamp: z.string(),
  session_id: z.string().optional(),
  metadata: z.record(z.any()).optional(),
})

export type Message = z.infer<typeof MessageSchema>

// Streaming response types
export const StreamingChunkSchema = z.object({
  type: z.enum(['content', 'tool_call', 'tool_result', 'error', 'done']),
  content: z.string().optional(),
  tool_call: z.object({
    id: z.string(),
    name: z.string(),
    input: z.record(z.any()),
  }).optional(),
  tool_result: z.object({
    id: z.string(),
    result: z.any(),
    error: z.string().optional(),
  }).optional(),
  error: z.string().optional(),
  metadata: z.record(z.any()).optional(),
})

export type StreamingChunk = z.infer<typeof StreamingChunkSchema>

// Chat history and session types
export interface ChatSession {
  id: string
  messages: Message[]
  created_at: string
  updated_at: string
  metadata?: Record<string, any>
}

export interface MessageThread {
  messages: Message[]
  isStreaming: boolean
  currentStreamingMessage?: Partial<Message>
}