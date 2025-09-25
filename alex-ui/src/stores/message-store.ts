import { create } from 'zustand'
import { immer } from 'zustand/middleware/immer'
import { MessageStore } from './types'
import { Message, ToolCall, StreamingState } from '@/types'
import { apiClient } from '@/services/api-client'
import { webSocketClient } from '@/services/websocket-client'

export const useMessageStore = create<MessageStore>()(
  immer((set, get) => ({
    // Initial state
    messageThread: {
      messages: [],
      isStreaming: false,
      currentStreamingMessage: undefined,
    },
    activeToolCalls: new Map(),
    streamingState: {
      isActive: false,
      phase: 'thinking',
    },

    // Message actions
    addMessage: (message: Message) => {
      set(state => {
        state.messageThread.messages.push(message)
      })
    },

    updateMessage: (id: string, updates: Partial<Message>) => {
      set(state => {
        const message = state.messageThread.messages.find(m => m.id === id)
        if (message) {
          Object.assign(message, updates)
        }
      })
    },

    removeMessage: (id: string) => {
      set(state => {
        state.messageThread.messages = state.messageThread.messages.filter(
          m => m.id !== id
        )
      })
    },

    clearMessages: () => {
      set(state => {
        state.messageThread.messages = []
        state.messageThread.currentStreamingMessage = undefined
        state.messageThread.isStreaming = false
      })
    },

    // Streaming actions
    startStreaming: (phase: StreamingState['phase']) => {
      set(state => {
        state.streamingState.isActive = true
        state.streamingState.phase = phase
        state.streamingState.progress = 0
        state.messageThread.isStreaming = true
      })
    },

    updateStreamingProgress: (progress: number, estimatedTimeMs?: number) => {
      set(state => {
        state.streamingState.progress = progress
        if (estimatedTimeMs !== undefined) {
          state.streamingState.estimatedTimeMs = estimatedTimeMs
        }
      })
    },

    stopStreaming: () => {
      set(state => {
        state.streamingState.isActive = false
        state.streamingState.progress = undefined
        state.streamingState.estimatedTimeMs = undefined
        state.messageThread.isStreaming = false
        state.messageThread.currentStreamingMessage = undefined
      })
    },

    // Tool call actions
    addToolCall: (toolCall: ToolCall) => {
      set(state => {
        state.activeToolCalls.set(toolCall.id, toolCall)
      })
    },

    updateToolCall: (id: string, updates: Partial<ToolCall>) => {
      set(state => {
        const toolCall = state.activeToolCalls.get(id)
        if (toolCall) {
          Object.assign(toolCall, updates)
          state.activeToolCalls.set(id, toolCall)
        }
      })
    },

    removeToolCall: (id: string) => {
      set(state => {
        state.activeToolCalls.delete(id)
      })
    },

    clearToolCalls: () => {
      set(state => {
        state.activeToolCalls.clear()
      })
    },

    // Message sending
    sendMessage: async (content: string) => {
      // Create user message
      const userMessage: Message = {
        id: crypto.randomUUID(),
        role: 'user',
        content: [{ type: 'text', text: content }],
        timestamp: new Date().toISOString(),
      }

      // Add user message immediately
      get().addMessage(userMessage)

      try {
        // Start streaming
        get().startStreaming('thinking')

        // Send via WebSocket for streaming response
        if (webSocketClient.isConnected()) {
          webSocketClient.send({
            type: 'message',
            data: {
              message: content,
              stream: true,
            },
          })
        } else {
          // Fallback to HTTP API
          const response = await apiClient.sendMessage({
            message: content,
            stream: false,
          })

          get().addMessage(response.message)
          get().stopStreaming()
        }
      } catch (error) {
        console.error('Failed to send message:', error)
        get().stopStreaming()
        throw error
      }
    },
  }))
)