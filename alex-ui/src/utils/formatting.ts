import chalk from 'chalk'

export function formatTimestamp(timestamp: string | Date): string {
  const date = typeof timestamp === 'string' ? new Date(timestamp) : timestamp
  return date.toLocaleTimeString('en-US', {
    hour12: false,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function formatDuration(durationMs: number): string {
  if (durationMs < 1000) {
    return `${durationMs}ms`
  }
  if (durationMs < 60000) {
    return `${(durationMs / 1000).toFixed(1)}s`
  }
  const minutes = Math.floor(durationMs / 60000)
  const seconds = Math.floor((durationMs % 60000) / 1000)
  return `${minutes}m ${seconds}s`
}

export function formatFileSize(bytes: number): string {
  const units = ['B', 'KB', 'MB', 'GB']
  let size = bytes
  let unitIndex = 0

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024
    unitIndex++
  }

  return `${size.toFixed(1)} ${units[unitIndex]}`
}

export function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text
  }
  return text.slice(0, maxLength - 3) + '...'
}

export function wrapText(text: string, width: number): string[] {
  const words = text.split(' ')
  const lines: string[] = []
  let currentLine = ''

  for (const word of words) {
    if ((currentLine + ' ' + word).length > width) {
      if (currentLine) {
        lines.push(currentLine)
        currentLine = word
      } else {
        // Word is longer than width, force break
        lines.push(word.slice(0, width))
        currentLine = word.slice(width)
      }
    } else {
      currentLine = currentLine ? currentLine + ' ' + word : word
    }
  }

  if (currentLine) {
    lines.push(currentLine)
  }

  return lines
}

// Terminal colors and styling
export const colors = {
  primary: chalk.blue,
  secondary: chalk.cyan,
  success: chalk.green,
  warning: chalk.yellow,
  error: chalk.red,
  muted: chalk.gray,
  accent: chalk.magenta,
  info: chalk.blue,

  // Tool-specific colors
  toolCall: chalk.yellow,
  toolResult: chalk.green,
  toolError: chalk.red,

  // Message role colors
  user: chalk.blue,
  assistant: chalk.green,
  system: chalk.gray,
  tool: chalk.yellow,
}

export function colorizeRole(role: string): string {
  switch (role) {
    case 'user':
      return 'cyan'
    case 'assistant':
      return 'green'
    case 'system':
      return 'gray'
    case 'tool':
      return 'yellow'
    default:
      return 'white'
  }
}

export function formatProgress(progress: number): string {
  const width = 20
  const filled = Math.floor((progress / 100) * width)
  const empty = width - filled
  return `[${'█'.repeat(filled)}${' '.repeat(empty)}] ${progress.toFixed(1)}%`
}
