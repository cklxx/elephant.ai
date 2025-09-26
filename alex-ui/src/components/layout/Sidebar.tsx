import React from 'react'
import { Box, Text } from 'ink'
import { useSessionStore, useAppStore } from '@/stores'
import { ToolCallDisplay } from '@/components/chat'

export interface SidebarProps {
  width?: number
  showSessions?: boolean
  showTools?: boolean
  showToolCalls?: boolean
}

export const Sidebar: React.FC<SidebarProps> = ({
  width = 30,
  showSessions = true,
  showTools = true,
  showToolCalls = true,
}) => {
  const { sessionList, currentSession } = useSessionStore()
  const { availableTools } = useAppStore()

  return (
    <Box
      flexDirection="column"
      width={width}
      borderStyle="round"
      borderColor="gray"
      padding={1}
    >
      {/* Sessions section */}
      {showSessions && (
        <Box flexDirection="column" marginBottom={2}>
          <Text color="blue" bold>
            Sessions
          </Text>
          <Box flexDirection="column" marginTop={1}>
            {sessionList.length === 0 ? (
              <Text color="gray" dimColor>
                No sessions
              </Text>
            ) : (
              sessionList.slice(0, 5).map(session => (
                <Box key={session.id} marginBottom={1}>
                  <Text
                    color={
                      session.id === currentSession?.state.id
                        ? 'green'
                        : 'white'
                    }
                    bold={session.id === currentSession?.state.id}
                  >
                    {session.id.slice(0, 8)}...
                  </Text>
                  <Box marginLeft={1}>
                    <Text color="gray" dimColor>
                      ({session.message_count} msgs)
                    </Text>
                  </Box>
                </Box>
              ))
            )}
          </Box>
        </Box>
      )}

      {/* Available tools section */}
      {showTools && (
        <Box flexDirection="column" marginBottom={2}>
          <Text color="blue" bold>
            Available Tools
          </Text>
          <Box flexDirection="column" marginTop={1}>
            {availableTools.length === 0 ? (
              <Text color="gray" dimColor>
                Loading tools...
              </Text>
            ) : (
              availableTools.slice(0, 8).map(tool => (
                <Box key={tool.name} marginBottom={1}>
                  <Text color="yellow">{tool.name}</Text>
                  {tool.risk_level && (
                    <Box marginLeft={1}>
                      <Text
                        color={
                          tool.risk_level === 'high'
                            ? 'red'
                            : tool.risk_level === 'medium'
                              ? 'yellow'
                              : 'green'
                        }
                        dimColor
                      >
                        {tool.risk_level}
                      </Text>
                    </Box>
                  )}
                </Box>
              ))
            )}
            {availableTools.length > 8 && (
              <Text color="gray" dimColor>
                ... and {availableTools.length - 8} more
              </Text>
            )}
          </Box>
        </Box>
      )}

      {/* Active tool calls section */}
      {showToolCalls && (
        <Box flexDirection="column">
          <ToolCallDisplay showCompleted={false} maxItems={3} />
        </Box>
      )}
    </Box>
  )
}
