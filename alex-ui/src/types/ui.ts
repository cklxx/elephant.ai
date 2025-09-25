// UI component props and state types

export interface BaseComponentProps {
  className?: string
  testId?: string
}

// Layout types
export interface LayoutConfig {
  showHeader: boolean
  showSidebar: boolean
  showFooter: boolean
  theme: 'light' | 'dark' | 'auto'
}

// Input component types
export interface InputState {
  value: string
  cursorPosition: number
  selectionStart?: number
  selectionEnd?: number
  history: string[]
  historyIndex: number
  isMultiline: boolean
}

export interface InputConfig {
  maxLength?: number
  multilineThreshold: number
  enableHistory: boolean
  historyLimit: number
  enableAutoComplete: boolean
  shortcuts: Record<string, () => void>
}

// Message display types
export interface MessageDisplayConfig {
  showTimestamps: boolean
  showMetadata: boolean
  enableSyntaxHighlighting: boolean
  maxContentLength?: number
  collapseThreshold: number
}

// Tool display types
export interface ToolDisplayConfig {
  showDetails: boolean
  showDuration: boolean
  collapseCompleted: boolean
  maxOutputLength?: number
}

// Streaming indicator types
export interface StreamingState {
  isActive: boolean
  phase: 'thinking' | 'acting' | 'observing' | 'responding'
  progress?: number
  estimatedTimeMs?: number
}

// Error display types
export interface ErrorState {
  message: string
  code?: string
  details?: Record<string, any>
  recoverable: boolean
  timestamp: string
}

// Notification types
export interface Notification {
  id: string
  type: 'info' | 'success' | 'warning' | 'error'
  title: string
  message?: string
  duration?: number
  persistent?: boolean
  actions?: Array<{
    label: string
    action: () => void
  }>
}

// Keyboard shortcuts
export interface KeyboardShortcut {
  key: string
  modifiers?: string[]
  description: string
  action: () => void
  enabled: boolean
}

// Theme configuration
export interface ThemeConfig {
  colors: {
    primary: string
    secondary: string
    background: string
    foreground: string
    muted: string
    accent: string
    error: string
    warning: string
    success: string
    info: string
  }
  spacing: {
    xs: number
    sm: number
    md: number
    lg: number
    xl: number
  }
  typography: {
    fontFamily: string
    fontSize: {
      xs: number
      sm: number
      md: number
      lg: number
      xl: number
    }
  }
}