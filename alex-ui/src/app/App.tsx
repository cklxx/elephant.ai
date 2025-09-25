import React from 'react'
import { Box, useInput } from 'ink'
import { AppProviders } from './providers'
import { MainLayout } from '@/components/layout'
import { MessageList } from '@/components/chat'
import { CommandInput } from '@/components/input'
import { useUIStore, useSessionStore, useAppStore } from '@/stores'
import { parseKeyPress, KeyboardHandler } from '@/utils/keyboard'

export interface AppProps {
  config?: {
    apiUrl?: string
    wsUrl?: string
    enableWebSocket?: boolean
    autoCreateSession?: boolean
  }
}

const AppContent: React.FC = () => {
  const { clearError, toggleSidebar, toggleHeader } = useUIStore()
  const { createSession, currentSession } = useSessionStore()
  const { isConnected } = useAppStore()

  const keyboardHandler = React.useMemo(() => {
    const handler = new KeyboardHandler()

    // Add custom key bindings
    handler.addBinding({
      key: 'l',
      ctrl: true,
      description: 'Clear error',
      action: () => clearError(),
    })

    handler.addBinding({
      key: 'n',
      ctrl: true,
      description: 'New session',
      action: () => {
        if (isConnected) {
          createSession().catch(console.error)
        }
      },
    })

    handler.addBinding({
      key: 'b',
      ctrl: true,
      description: 'Toggle sidebar',
      action: () => toggleSidebar(),
    })

    handler.addBinding({
      key: 'h',
      ctrl: true,
      description: 'Toggle header',
      action: () => toggleHeader(),
    })

    return handler
  }, [clearError, createSession, isConnected, toggleSidebar, toggleHeader])

  // Global key handler
  useInput((input) => {
    const keyPress = parseKeyPress(input)
    keyboardHandler.handleKeyPress(keyPress)
  })

  // Auto-create session if needed
  React.useEffect(() => {
    if (isConnected && !currentSession) {
      createSession().catch(console.error)
    }
  }, [isConnected, currentSession, createSession])

  return (
    <MainLayout showSidebar={false}>
      <Box flexDirection="column" flexGrow={1}>
        {/* Main chat area */}
        <Box flexGrow={1}>
          <MessageList maxHeight={25} maxWidth={100} />
        </Box>

        {/* Input area */}
        <Box marginTop={1}>
          <CommandInput
            placeholder="Type your message and press Enter..."
            disabled={!isConnected || !currentSession}
          />
        </Box>
      </Box>
    </MainLayout>
  )
}

export const App: React.FC<AppProps> = ({
  config = {
    apiUrl: process.env.API_URL || 'http://localhost:8080/api/v1',
    wsUrl: process.env.WS_URL || 'ws://localhost:8080/ws',
    enableWebSocket: true,
    autoCreateSession: true,
  },
}) => {
  return (
    <AppProviders config={config}>
      <AppContent />
    </AppProviders>
  )
}