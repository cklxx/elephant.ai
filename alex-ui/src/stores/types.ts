import {
  Message,
  MessageThread,
  Session,
  SessionState,
  SessionConfig,
  ToolCall,
  ToolDefinition,
  ErrorState,
  Notification,
  InputState,
  StreamingState,
} from '@/types'

// App-wide state
export interface AppStore {
  // Connection state
  isConnected: boolean
  connectionError?: string

  // Available tools
  availableTools: ToolDefinition[]

  // Global notifications
  notifications: Notification[]

  // Actions
  setConnected: (connected: boolean) => void
  setConnectionError: (error?: string) => void
  setAvailableTools: (tools: ToolDefinition[]) => void
  addNotification: (notification: Omit<Notification, 'id'>) => void
  removeNotification: (id: string) => void
  clearNotifications: () => void
}

// Session management state
export interface SessionStore {
  // Current session
  currentSession?: Session
  sessionList: SessionState[]

  // Session operations
  createSession: (config?: Partial<SessionConfig>) => Promise<void>
  loadSession: (id: string) => Promise<void>
  saveSession: () => Promise<void>
  deleteSession: (id: string) => Promise<void>
  listSessions: () => Promise<void>
  resumeSession: (id: string) => Promise<void>

  // Session state updates
  updateSessionState: (updates: Partial<SessionState>) => void
  updateSessionConfig: (updates: Partial<SessionConfig>) => void
}

// Message and chat state
export interface MessageStore {
  // Message thread
  messageThread: MessageThread

  // Active tool calls
  activeToolCalls: Map<string, ToolCall>

  // Streaming state
  streamingState: StreamingState

  // Actions
  addMessage: (message: Message) => void
  updateMessage: (id: string, updates: Partial<Message>) => void
  removeMessage: (id: string) => void
  clearMessages: () => void

  // Streaming actions
  startStreaming: (phase: StreamingState['phase']) => void
  updateStreamingProgress: (progress: number, estimatedTimeMs?: number) => void
  stopStreaming: () => void

  // Tool call actions
  addToolCall: (toolCall: ToolCall) => void
  updateToolCall: (id: string, updates: Partial<ToolCall>) => void
  removeToolCall: (id: string) => void
  clearToolCalls: () => void

  // Message sending
  sendMessage: (content: string) => Promise<void>
}

// UI state and preferences
export interface UIStore {
  // Input state
  inputState: InputState

  // Error state
  errorState?: ErrorState

  // UI preferences
  showTimestamps: boolean
  showMetadata: boolean
  enableSyntaxHighlighting: boolean
  theme: 'light' | 'dark' | 'auto'

  // Layout state
  sidebarOpen: boolean
  headerVisible: boolean

  // Actions
  updateInputState: (updates: Partial<InputState>) => void
  setError: (error: ErrorState) => void
  clearError: () => void

  // UI preferences
  toggleTimestamps: () => void
  toggleMetadata: () => void
  toggleSyntaxHighlighting: () => void
  setTheme: (theme: 'light' | 'dark' | 'auto') => void

  // Layout actions
  toggleSidebar: () => void
  toggleHeader: () => void
}