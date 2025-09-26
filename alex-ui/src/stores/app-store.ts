import { create } from 'zustand'
import { immer } from 'zustand/middleware/immer'
import { AppStore } from './types'
import { ToolDefinition, Notification } from '@/types'

export const useAppStore = create<AppStore>()(
  immer((set, get) => ({
    // Initial state
    isConnected: false,
    connectionError: undefined,
    availableTools: [],
    notifications: [],

    // Connection actions
    setConnected: (connected: boolean) => {
      set(state => {
        state.isConnected = connected
        if (connected) {
          state.connectionError = undefined
        }
      })
    },

    setConnectionError: (error?: string) => {
      set(state => {
        state.connectionError = error
        if (error) {
          state.isConnected = false
        }
      })
    },

    // Tools actions
    setAvailableTools: (tools: ToolDefinition[]) => {
      set(state => {
        state.availableTools = tools
      })
    },

    // Notification actions
    addNotification: (notification: Omit<Notification, 'id'>) => {
      const id = crypto.randomUUID()
      set(state => {
        state.notifications.push({ ...notification, id })
      })

      // Auto-remove non-persistent notifications
      if (!notification.persistent) {
        const duration = notification.duration || 5000
        setTimeout(() => {
          get().removeNotification(id)
        }, duration)
      }
    },

    removeNotification: (id: string) => {
      set(state => {
        const index = state.notifications.findIndex(n => n.id === id)
        if (index >= 0) {
          state.notifications.splice(index, 1)
        }
      })
    },

    clearNotifications: () => {
      set(state => {
        state.notifications = []
      })
    },
  }))
)
