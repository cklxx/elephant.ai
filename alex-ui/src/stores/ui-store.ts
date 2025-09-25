import { create } from 'zustand'
import { immer } from 'zustand/middleware/immer'
import { persist } from 'zustand/middleware'
import { UIStore } from './types'
import { InputState, ErrorState } from '@/types'

const initialInputState: InputState = {
  value: '',
  cursorPosition: 0,
  history: [],
  historyIndex: -1,
  isMultiline: false,
}

export const useUIStore = create<UIStore>()(
  persist(
    immer((set) => ({
      // Initial state
      inputState: initialInputState,
      errorState: undefined,

      // UI preferences (persisted)
      showTimestamps: true,
      showMetadata: false,
      enableSyntaxHighlighting: true,
      theme: 'auto' as const,

      // Layout state (not persisted)
      sidebarOpen: false,
      headerVisible: true,

      // Input actions
      updateInputState: (updates: Partial<InputState>) => {
        set(state => {
          Object.assign(state.inputState, updates)
        })
      },

      // Error actions
      setError: (error: ErrorState) => {
        set(state => {
          state.errorState = error
        })
      },

      clearError: () => {
        set(state => {
          state.errorState = undefined
        })
      },

      // UI preference actions
      toggleTimestamps: () => {
        set(state => {
          state.showTimestamps = !state.showTimestamps
        })
      },

      toggleMetadata: () => {
        set(state => {
          state.showMetadata = !state.showMetadata
        })
      },

      toggleSyntaxHighlighting: () => {
        set(state => {
          state.enableSyntaxHighlighting = !state.enableSyntaxHighlighting
        })
      },

      setTheme: (theme: 'light' | 'dark' | 'auto') => {
        set(state => {
          state.theme = theme
        })
      },

      // Layout actions
      toggleSidebar: () => {
        set(state => {
          state.sidebarOpen = !state.sidebarOpen
        })
      },

      toggleHeader: () => {
        set(state => {
          state.headerVisible = !state.headerVisible
        })
      },
    })),
    {
      name: 'alex-ui-preferences',
      partialize: state => ({
        showTimestamps: state.showTimestamps,
        showMetadata: state.showMetadata,
        enableSyntaxHighlighting: state.enableSyntaxHighlighting,
        theme: state.theme,
      }),
    }
  )
)