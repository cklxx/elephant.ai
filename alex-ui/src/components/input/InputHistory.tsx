import React from 'react'
import { Box, Text } from 'ink'
import { useUIStore } from '@/stores'
import { formatTimestamp } from '@/utils/formatting'

export interface InputHistoryProps {
  maxItems?: number
  showTimestamps?: boolean
}

export const InputHistory: React.FC<InputHistoryProps> = ({
  maxItems = 10,
  showTimestamps = false,
}) => {
  const { inputState } = useUIStore()

  const displayHistory = React.useMemo(() => {
    return inputState.history.slice(0, maxItems)
  }, [inputState.history, maxItems])

  if (displayHistory.length === 0) {
    return (
      <Box>
        <Text color="gray">No command history</Text>
      </Box>
    )
  }

  return (
    <Box flexDirection="column">
      <Box marginBottom={1}>
        <Text color="blue" bold>
          Command History
        </Text>
      </Box>

      {displayHistory.map((command, index) => (
        <Box key={index} marginBottom={index < displayHistory.length - 1 ? 1 : 0}>
          <Box width={4}>
            <Text color="gray">{String(index + 1).padStart(2)}</Text>
          </Box>
          <Text> </Text>
          <Box flexGrow={1}>
            <Text>{command}</Text>
          </Box>
          {showTimestamps && (
            <Box marginLeft={2}>
              <Text color="gray">{formatTimestamp(new Date())}</Text>
            </Box>
          )}
        </Box>
      ))}

      {inputState.history.length > maxItems && (
        <Box marginTop={1}>
          <Text color="gray">
            ... and {inputState.history.length - maxItems} more commands
          </Text>
        </Box>
      )}
    </Box>
  )
}