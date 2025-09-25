import { z } from 'zod'

// Tool execution status
export const ToolStatusSchema = z.enum([
  'pending',
  'running',
  'completed',
  'failed',
  'cancelled'
])
export type ToolStatus = z.infer<typeof ToolStatusSchema>

// Tool definition schema
export const ToolParameterSchema = z.object({
  type: z.string(),
  description: z.string(),
  required: z.boolean().optional(),
  enum: z.array(z.string()).optional(),
  default: z.any().optional(),
})

export const ToolDefinitionSchema = z.object({
  name: z.string(),
  description: z.string(),
  parameters: z.record(ToolParameterSchema),
  required_parameters: z.array(z.string()).optional(),
  category: z.string().optional(),
  risk_level: z.enum(['low', 'medium', 'high']).optional(),
})

export type ToolDefinition = z.infer<typeof ToolDefinitionSchema>

// Tool call execution
export const ToolCallSchema = z.object({
  id: z.string(),
  name: z.string(),
  input: z.record(z.any()),
  status: ToolStatusSchema,
  result: z.any().optional(),
  error: z.string().optional(),
  started_at: z.string(),
  completed_at: z.string().optional(),
  duration_ms: z.number().optional(),
})

export type ToolCall = z.infer<typeof ToolCallSchema>

// Built-in tool categories
export const BUILTIN_TOOLS = {
  FILE_OPERATIONS: ['file_read', 'file_update', 'file_replace', 'file_list'],
  SHELL_EXECUTION: ['bash', 'code_execute'],
  SEARCH_ANALYSIS: ['grep', 'ripgrep', 'find'],
  TASK_MANAGEMENT: ['todo_read', 'todo_update'],
  WEB_INTEGRATION: ['web_search', 'web_fetch'],
  REASONING: ['think'],
} as const

export type BuiltinToolCategory = keyof typeof BUILTIN_TOOLS
export type BuiltinToolName = typeof BUILTIN_TOOLS[BuiltinToolCategory][number]