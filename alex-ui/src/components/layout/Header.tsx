import React from 'react'
import { Box, Text } from 'ink'
import { useAppStore, useSessionStore } from '@/stores'

export interface HeaderProps {
  title?: string
  showConnectionStatus?: boolean
  showSessionInfo?: boolean
}

export const Header: React.FC<HeaderProps> = ({
  title = 'ALEX - AI Code Agent',
  showConnectionStatus = true,
  showSessionInfo = true,
}) => {
  const { isConnected, connectionError } = useAppStore()
  const { currentSession } = useSessionStore()

  return (
    <Box
      justifyContent="space-between"
      padding={1}
      borderStyle="round"
      borderColor="blue"
    >
      {/* Title */}
      <Box>
        <Text color="blue" bold>
          {title}
        </Text>
      </Box>

      {/* Status indicators */}
      <Box>
        {/* Connection status */}
        {showConnectionStatus && (
          <Box marginRight={2}>
            {isConnected ? (
              <Box gap={1}>
                <Text color="green">●</Text>
                <Text color="green">Connected</Text>
              </Box>
            ) : (
              <Box gap={1}>
                <Text color="red">●</Text>
                <Text color="red">
                  {connectionError ? 'Error' : 'Disconnected'}
                </Text>
              </Box>
            )}
          </Box>
        )}

        {/* Session info */}
        {showSessionInfo && currentSession && (
          <Box gap={1}>
            <Text color="cyan">Session: </Text>
            <Text color="white">{currentSession.state.id.slice(0, 8)}...</Text>
            <Text color="gray">
              ({currentSession.state.message_count} msgs)
            </Text>
          </Box>
        )}
      </Box>
    </Box>
  )
}
