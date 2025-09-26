import { create } from 'zustand'
import { immer } from 'zustand/middleware/immer'
import { SessionStore } from './types'
import { SessionState, SessionConfig } from '@/types'
import { apiClient } from '@/services/api-client'
import { connectionManager } from '@/services/connection-manager'

export const useSessionStore = create<SessionStore>()(
  immer((set, get) => ({
    // Initial state
    currentSession: undefined,
    sessionList: [],

    // Session operations
    createSession: async (config?: Partial<SessionConfig>) => {
      try {
        const session = await apiClient.createSession({ config })
        set(state => {
          state.currentSession = session
        })

        // 为新会话建立WebSocket连接
        await connectionManager.connectWebSocketForSession(session.state.id)
      } catch (error) {
        console.error('Failed to create session:', error)
        throw error
      }
    },

    loadSession: async (id: string) => {
      try {
        const response = await apiClient.loadSession(id)
        set(state => {
          state.currentSession = response.session
        })

        // 为加载的会话建立WebSocket连接
        await connectionManager.connectWebSocketForSession(id)
      } catch (error) {
        console.error('Failed to load session:', error)
        throw error
      }
    },

    saveSession: async () => {
      const { currentSession } = get()
      if (!currentSession) {
        throw new Error('No active session to save')
      }

      try {
        // In a real implementation, this would call the API
        // For now, we'll just simulate success
        console.log('Session saved:', currentSession.state.id)
      } catch (error) {
        console.error('Failed to save session:', error)
        throw error
      }
    },

    deleteSession: async (id: string) => {
      try {
        // 如果删除的是当前会话，断开WebSocket连接
        const { currentSession } = get()
        if (currentSession?.state.id === id) {
          connectionManager.disconnectWebSocket()
        }

        await apiClient.deleteSession(id)
        set(state => {
          if (state.currentSession?.state.id === id) {
            state.currentSession = undefined
          }
          state.sessionList = state.sessionList.filter(s => s.id !== id)
        })
      } catch (error) {
        console.error('Failed to delete session:', error)
        throw error
      }
    },

    listSessions: async () => {
      try {
        const sessions = await apiClient.listSessions()
        set(state => {
          state.sessionList = sessions
        })
      } catch (error) {
        console.error('Failed to list sessions:', error)
        throw error
      }
    },

    resumeSession: async (id: string) => {
      try {
        await get().loadSession(id)
      } catch (error) {
        console.error('Failed to resume session:', error)
        throw error
      }
    },

    // Session state updates
    updateSessionState: (updates: Partial<SessionState>) => {
      set(state => {
        if (state.currentSession) {
          Object.assign(state.currentSession.state, updates)
        }
      })
    },

    updateSessionConfig: (updates: Partial<SessionConfig>) => {
      set(state => {
        if (state.currentSession) {
          Object.assign(state.currentSession.config, updates)
        }
      })
    },
  }))
)
