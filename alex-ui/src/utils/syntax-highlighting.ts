import hljs from 'highlight.js'
import chalk from 'chalk'

// Language detection patterns
const LANGUAGE_PATTERNS = {
  javascript: /```(?:js|javascript)\s*\n([\s\S]*?)\n```/gi,
  typescript: /```(?:ts|typescript)\s*\n([\s\S]*?)\n```/gi,
  python: /```(?:py|python)\s*\n([\s\S]*?)\n```/gi,
  bash: /```(?:bash|sh|shell)\s*\n([\s\S]*?)\n```/gi,
  json: /```(?:json)\s*\n([\s\S]*?)\n```/gi,
  yaml: /```(?:yaml|yml)\s*\n([\s\S]*?)\n```/gi,
  go: /```(?:go|golang)\s*\n([\s\S]*?)\n```/gi,
  sql: /```(?:sql)\s*\n([\s\S]*?)\n```/gi,
  markdown: /```(?:md|markdown)\s*\n([\s\S]*?)\n```/gi,
}

// Terminal color mapping for syntax highlighting
const SYNTAX_COLORS = {
  keyword: chalk.blue,
  string: chalk.green,
  number: chalk.cyan,
  comment: chalk.gray,
  function: chalk.yellow,
  variable: chalk.white,
  operator: chalk.red,
  punctuation: chalk.white,
  class: chalk.magenta,
  type: chalk.cyan,
}

export function detectLanguage(code: string): string {
  // Try to detect language from code patterns
  for (const [lang, pattern] of Object.entries(LANGUAGE_PATTERNS)) {
    if (pattern.test(code)) {
      return lang
    }
  }

  // Fallback to hljs auto-detection
  try {
    const result = hljs.highlightAuto(code)
    return result.language || 'plaintext'
  } catch {
    return 'plaintext'
  }
}

export function highlightCode(code: string, language?: string): string {
  try {
    const lang = language || detectLanguage(code)
    const highlighted = hljs.highlight(code, { language: lang })

    // Convert HTML to terminal colors
    return convertHtmlToTerminalColors(highlighted.value)
  } catch (error) {
    console.warn('Syntax highlighting failed:', error)
    return code
  }
}

function convertHtmlToTerminalColors(html: string): string {
  // Simple HTML to terminal color conversion
  // This is a basic implementation - in production you might want to use a more sophisticated parser

  let result = html

  // Replace common hljs classes with terminal colors
  result = result.replace(
    /<span class="hljs-keyword"[^>]*>(.*?)<\/span>/g,
    (_, text) => SYNTAX_COLORS.keyword(text)
  )
  result = result.replace(
    /<span class="hljs-string"[^>]*>(.*?)<\/span>/g,
    (_, text) => SYNTAX_COLORS.string(text)
  )
  result = result.replace(
    /<span class="hljs-number"[^>]*>(.*?)<\/span>/g,
    (_, text) => SYNTAX_COLORS.number(text)
  )
  result = result.replace(
    /<span class="hljs-comment"[^>]*>(.*?)<\/span>/g,
    (_, text) => SYNTAX_COLORS.comment(text)
  )
  result = result.replace(
    /<span class="hljs-function"[^>]*>(.*?)<\/span>/g,
    (_, text) => SYNTAX_COLORS.function(text)
  )
  result = result.replace(
    /<span class="hljs-variable"[^>]*>(.*?)<\/span>/g,
    (_, text) => SYNTAX_COLORS.variable(text)
  )
  result = result.replace(
    /<span class="hljs-operator"[^>]*>(.*?)<\/span>/g,
    (_, text) => SYNTAX_COLORS.operator(text)
  )

  // Remove remaining HTML tags
  result = result.replace(/<[^>]*>/g, '')

  // Decode HTML entities
  result = result
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;/g, "'")

  return result
}

export function formatCodeBlock(
  code: string,
  language?: string,
  lineNumbers = false
): string {
  const highlighted = highlightCode(code, language)
  const lines = highlighted.split('\n')

  if (!lineNumbers) {
    return highlighted
  }

  const maxLineNumberWidth = lines.length.toString().length

  return lines
    .map((line, index) => {
      const lineNumber = (index + 1)
        .toString()
        .padStart(maxLineNumberWidth, ' ')
      return `${chalk.gray(lineNumber)} │ ${line}`
    })
    .join('\n')
}

export function extractCodeBlocks(
  text: string
): Array<{ code: string; language?: string }> {
  const codeBlocks: Array<{ code: string; language?: string }> = []

  // Extract fenced code blocks
  const fencedRegex = /```(\w+)?\s*\n([\s\S]*?)\n```/g
  let match

  while ((match = fencedRegex.exec(text)) !== null) {
    codeBlocks.push({
      code: match[2],
      language: match[1],
    })
  }

  // Extract inline code
  const inlineRegex = /`([^`]+)`/g
  while ((match = inlineRegex.exec(text)) !== null) {
    if (match[1].length > 50) {
      // Only highlight longer inline code
      codeBlocks.push({
        code: match[1],
      })
    }
  }

  return codeBlocks
}

export function isCodeLikeContent(text: string): boolean {
  // Heuristics to detect if content looks like code
  const codeIndicators = [
    /function\s+\w+\s*\(/,
    /class\s+\w+/,
    /import\s+.*from/,
    /export\s+(default\s+)?/,
    /def\s+\w+\s*\(/,
    /console\.log\(/,
    /\.addEventListener\(/,
    /\$\(.*\)/,
    /{[\s\S]*}/,
    /\[[\s\S]*\]/,
  ]

  return codeIndicators.some(pattern => pattern.test(text))
}
