import { z } from 'zod'

// Validation utilities

export function isValidUrl(url: string): boolean {
  try {
    new URL(url)
    return true
  } catch {
    return false
  }
}

export function isValidJson(str: string): boolean {
  try {
    JSON.parse(str)
    return true
  } catch {
    return false
  }
}

export function sanitizeInput(input: string): string {
  // Remove ANSI escape codes
  // eslint-disable-next-line no-control-regex
  const ansiEscapePattern = /\u001b\[[0-9;]*m/g
  let sanitized = input.replace(ansiEscapePattern, '')

  // Remove null bytes
  sanitized = sanitized.replace(/\0/g, '')

  // Normalize line endings
  sanitized = sanitized.replace(/\r\n/g, '\n').replace(/\r/g, '\n')

  return sanitized
}

export function validateSessionId(id: string): boolean {
  // Session ID should be a valid UUID or alphanumeric string
  const uuidPattern =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
  const alphanumericPattern = /^[a-zA-Z0-9_-]+$/

  return uuidPattern.test(id) || alphanumericPattern.test(id)
}

export function validateToolName(name: string): boolean {
  // Tool names should be alphanumeric with underscores
  const pattern = /^[a-zA-Z][a-zA-Z0-9_]*$/
  return pattern.test(name) && name.length <= 50
}

export function validateMessageContent(content: string): boolean {
  // Basic message validation
  return (
    content.length > 0 &&
    content.length <= 50000 && // Reasonable limit
    !content.includes('\0') // No null bytes
  )
}

// Schema validators using Zod
export const ConfigSchemaValidator = z.object({
  apiUrl: z.string().url().optional(),
  wsUrl: z.string().url().optional(),
  enableWebSocket: z.boolean().optional(),
  autoReconnect: z.boolean().optional(),
  healthCheckInterval: z.number().positive().optional(),
  theme: z.enum(['light', 'dark', 'auto']).optional(),
  enableSyntaxHighlighting: z.boolean().optional(),
  maxHistorySize: z.number().positive().optional(),
})

export type ConfigSchema = z.infer<typeof ConfigSchemaValidator>

export const EnvironmentValidator = z.object({
  NODE_ENV: z.enum(['development', 'production', 'test']).optional(),
  API_URL: z.string().url().optional(),
  WS_URL: z.string().url().optional(),
  DEBUG: z.string().optional(),
})

export function validateEnvironment(): z.infer<typeof EnvironmentValidator> {
  return EnvironmentValidator.parse(process.env)
}

export function validateConfig(config: unknown): ConfigSchema {
  return ConfigSchemaValidator.parse(config)
}

// Error formatting utilities
export function formatValidationError(error: z.ZodError): string {
  const issues = error.issues.map(issue => {
    const path = issue.path.length > 0 ? issue.path.join('.') : 'root'
    return `${path}: ${issue.message}`
  })

  return `Validation error:\n${issues.join('\n')}`
}

export function isValidCommand(command: string): boolean {
  // Basic command validation
  const trimmed = command.trim()
  return (
    trimmed.length > 0 &&
    trimmed.length <= 10000 &&
    !trimmed.includes('\0') &&
    // Not just whitespace
    /\S/.test(trimmed)
  )
}

export function normalizeWhitespace(text: string): string {
  // Normalize multiple spaces to single spaces
  return text.replace(/\s+/g, ' ').trim()
}

export function escapeHtml(text: string): string {
  const htmlEscapes: Record<string, string> = {
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#x27;',
    '/': '&#x2F;',
  }

  return text.replace(/[&<>"'/]/g, match => htmlEscapes[match])
}

export function unescapeHtml(text: string): string {
  const htmlUnescapes: Record<string, string> = {
    '&amp;': '&',
    '&lt;': '<',
    '&gt;': '>',
    '&quot;': '"',
    '&#x27;': "'",
    '&#x2F;': '/',
  }

  return text.replace(
    /&(?:amp|lt|gt|quot|#x27|#x2F);/g,
    match => htmlUnescapes[match]
  )
}
