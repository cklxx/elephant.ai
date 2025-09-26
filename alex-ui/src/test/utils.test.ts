import { describe, it, expect } from 'vitest'
import {
  formatTimestamp,
  formatDuration,
  truncateText,
  wrapText,
  formatProgress,
} from '@/utils/formatting'
import {
  validateSessionId,
  validateToolName,
  validateMessageContent,
  sanitizeInput,
} from '@/utils/validation'
import { parseKeyPress, keyMatches } from '@/utils/keyboard'

describe('Formatting Utils', () => {
  it('should format timestamps correctly', () => {
    const date = new Date('2023-01-01T12:30:45Z')
    const formatted = formatTimestamp(date)
    expect(formatted).toMatch(/\d{2}:\d{2}:\d{2}/)
  })

  it('should format durations correctly', () => {
    expect(formatDuration(500)).toBe('500ms')
    expect(formatDuration(1500)).toBe('1.5s')
    expect(formatDuration(65000)).toBe('1m 5s')
  })

  it('should truncate text properly', () => {
    expect(truncateText('Hello World', 5)).toBe('He...')
    expect(truncateText('Hello', 10)).toBe('Hello')
  })

  it('should wrap text correctly', () => {
    const lines = wrapText('This is a long line that should be wrapped', 10)
    expect(lines).toHaveLength(5)
    expect(lines[0]).toBe('This is a')
  })

  it('should format progress bars', () => {
    const progress = formatProgress(50)
    expect(progress).toContain('50.0%')
    expect(progress).toContain('█')
  })
})

describe('Validation Utils', () => {
  it('should validate session IDs', () => {
    expect(validateSessionId('123e4567-e89b-12d3-a456-426614174000')).toBe(true)
    expect(validateSessionId('valid_session_123')).toBe(true)
    expect(validateSessionId('invalid session!')).toBe(false)
  })

  it('should validate tool names', () => {
    expect(validateToolName('file_read')).toBe(true)
    expect(validateToolName('bash')).toBe(true)
    expect(validateToolName('123invalid')).toBe(false)
    expect(validateToolName('tool-with-dash')).toBe(false)
  })

  it('should validate message content', () => {
    expect(validateMessageContent('Hello world')).toBe(true)
    expect(validateMessageContent('')).toBe(false)
    expect(validateMessageContent('a'.repeat(60000))).toBe(false)
  })

  it('should sanitize input', () => {
    const input = 'Hello\u001b[31mWorld\u001b[0m\0test\r\n'
    const sanitized = sanitizeInput(input)
    expect(sanitized).toBe('HelloWorldtest\n')
  })
})

describe('Keyboard Utils', () => {
  it('should parse key presses correctly', () => {
    const enterKey = parseKeyPress('\r')
    expect(enterKey.name).toBe('return')

    const ctrlC = parseKeyPress('\u0003')
    expect(ctrlC.ctrl).toBe(true)
    expect(ctrlC.name).toBe('c')
  })

  it('should match key bindings correctly', () => {
    const keyPress = { name: 'c', ctrl: true }
    const binding = {
      key: 'c',
      ctrl: true,
      description: 'Copy',
      action: () => {},
    }

    expect(keyMatches(keyPress, binding)).toBe(true)

    const nonMatchingPress = { name: 'c', ctrl: false }
    expect(keyMatches(nonMatchingPress, binding)).toBe(false)
  })
})
