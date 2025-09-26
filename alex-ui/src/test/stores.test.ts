import { describe, it, expect, beforeEach } from 'vitest'
import { useAppStore } from '@/stores/app-store'
import { useUIStore } from '@/stores/ui-store'
import { ToolDefinition, Notification } from '@/types'

describe('App Store', () => {
  beforeEach(() => {
    useAppStore.setState({
      isConnected: false,
      connectionError: undefined,
      availableTools: [],
      notifications: [],
    })
  })

  it('should set connection status', () => {
    const { setConnected } = useAppStore.getState()

    setConnected(true)
    expect(useAppStore.getState().isConnected).toBe(true)

    setConnected(false)
    expect(useAppStore.getState().isConnected).toBe(false)
  })

  it('should manage connection errors', () => {
    const { setConnectionError } = useAppStore.getState()

    setConnectionError('Connection failed')
    expect(useAppStore.getState().connectionError).toBe('Connection failed')
    expect(useAppStore.getState().isConnected).toBe(false)

    setConnectionError(undefined)
    expect(useAppStore.getState().connectionError).toBeUndefined()
  })

  it('should manage available tools', () => {
    const tools: ToolDefinition[] = [
      {
        name: 'file_read',
        description: 'Read file contents',
        parameters: {},
      },
    ]

    const { setAvailableTools } = useAppStore.getState()
    setAvailableTools(tools)

    expect(useAppStore.getState().availableTools).toEqual(tools)
  })

  it('should manage notifications', () => {
    const { addNotification, removeNotification } = useAppStore.getState()

    const notification: Omit<Notification, 'id'> = {
      type: 'info',
      title: 'Test notification',
      message: 'This is a test',
    }

    addNotification(notification)

    const notifications = useAppStore.getState().notifications
    expect(notifications).toHaveLength(1)
    expect(notifications[0].title).toBe('Test notification')

    removeNotification(notifications[0].id)
    expect(useAppStore.getState().notifications).toHaveLength(0)
  })
})

describe('UI Store', () => {
  beforeEach(() => {
    useUIStore.setState({
      inputState: {
        value: '',
        cursorPosition: 0,
        history: [],
        historyIndex: -1,
        isMultiline: false,
      },
      errorState: undefined,
      showTimestamps: true,
      showMetadata: false,
      enableSyntaxHighlighting: true,
      theme: 'auto',
      sidebarOpen: false,
      headerVisible: true,
    })
  })

  it('should update input state', () => {
    const { updateInputState } = useUIStore.getState()

    updateInputState({ value: 'Hello', cursorPosition: 5 })

    const inputState = useUIStore.getState().inputState
    expect(inputState.value).toBe('Hello')
    expect(inputState.cursorPosition).toBe(5)
  })

  it('should manage error state', () => {
    const { setError, clearError } = useUIStore.getState()

    const error = {
      message: 'Test error',
      recoverable: true,
      timestamp: new Date().toISOString(),
    }

    setError(error)
    expect(useUIStore.getState().errorState).toEqual(error)

    clearError()
    expect(useUIStore.getState().errorState).toBeUndefined()
  })

  it('should toggle UI preferences', () => {
    const { toggleTimestamps, toggleSyntaxHighlighting, setTheme } =
      useUIStore.getState()

    toggleTimestamps()
    expect(useUIStore.getState().showTimestamps).toBe(false)

    toggleSyntaxHighlighting()
    expect(useUIStore.getState().enableSyntaxHighlighting).toBe(false)

    setTheme('dark')
    expect(useUIStore.getState().theme).toBe('dark')
  })

  it('should toggle layout state', () => {
    const { toggleSidebar, toggleHeader } = useUIStore.getState()

    toggleSidebar()
    expect(useUIStore.getState().sidebarOpen).toBe(true)

    toggleHeader()
    expect(useUIStore.getState().headerVisible).toBe(false)
  })
})
