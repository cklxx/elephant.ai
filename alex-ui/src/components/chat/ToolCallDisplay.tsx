import React from 'react'
import { Box, Text } from 'ink'
import { ToolCall } from '@/types'
import { useMessageStore } from '@/stores'
import { Spinner, ProgressBar } from '@/components/common'
import { formatDuration } from '@/utils/formatting'
import { formatCodeBlock } from '@/utils/syntax-highlighting'

export interface ToolCallDisplayProps {
  showCompleted?: boolean
  maxItems?: number
}

const STATUS_COLORS = {
  pending: 'gray',
  running: 'yellow',
  completed: 'green',
  failed: 'red',
  cancelled: 'orange',
} as const

const STATUS_ICONS = {
  pending: '⏳',
  running: '⚡',
  completed: '✅',
  failed: '❌',
  cancelled: '⚠️',
} as const

export const ToolCallDisplay: React.FC<ToolCallDisplayProps> = ({
  showCompleted = true,
  maxItems = 5,
}) => {
  const { activeToolCalls } = useMessageStore()

  const toolCallsArray = React.useMemo(() => {
    const calls = Array.from(activeToolCalls.values())

    // Filter based on showCompleted setting
    const filtered = showCompleted
      ? calls
      : calls.filter(call => call.status !== 'completed')

    // Sort by start time (most recent first)
    const sorted = filtered.sort(
      (a, b) =>
        new Date(b.started_at).getTime() - new Date(a.started_at).getTime()
    )

    // Limit number of items
    return sorted.slice(0, maxItems)
  }, [activeToolCalls, showCompleted, maxItems])

  if (toolCallsArray.length === 0) {
    return null
  }

  const renderToolCall = (toolCall: ToolCall) => {
    const statusColor = STATUS_COLORS[toolCall.status]
    const statusIcon = STATUS_ICONS[toolCall.status]

    return (
      <Box
        key={toolCall.id}
        flexDirection="column"
        borderStyle="round"
        borderColor={statusColor}
        padding={1}
        marginBottom={1}
      >
        {/* Tool call header */}
        <Box justifyContent="space-between">
          <Box>
            <Text>{statusIcon} </Text>
            <Text color={statusColor} bold>
              {toolCall.name}
            </Text>
          </Box>
          <Box gap={1}>
            {toolCall.status === 'running' && <Spinner type="dots" />}
            <Text color="gray">{toolCall.status.toUpperCase()}</Text>
          </Box>
        </Box>

        {/* Tool input */}
        {Object.keys(toolCall.input).length > 0 && (
          <Box flexDirection="column" marginTop={1}>
            <Text color="blue" bold>
              Input:
            </Text>
            <Box marginLeft={2}>
              <Text color="gray">
                {formatCodeBlock(
                  JSON.stringify(toolCall.input, null, 2),
                  'json'
                )}
              </Text>
            </Box>
          </Box>
        )}

        {/* Tool result */}
        {toolCall.result && (
          <Box flexDirection="column" marginTop={1}>
            <Text color="green" bold>
              Result:
            </Text>
            <Box marginLeft={2}>
              {typeof toolCall.result === 'string' ? (
                <Text>{toolCall.result}</Text>
              ) : (
                <Text color="gray">
                  {formatCodeBlock(
                    JSON.stringify(toolCall.result, null, 2),
                    'json'
                  )}
                </Text>
              )}
            </Box>
          </Box>
        )}

        {/* Tool error */}
        {toolCall.error && (
          <Box flexDirection="column" marginTop={1}>
            <Text color="red" bold>
              Error:
            </Text>
            <Box marginLeft={2}>
              <Text color="red">{toolCall.error}</Text>
            </Box>
          </Box>
        )}

        {/* Duration */}
        {toolCall.duration_ms && (
          <Box marginTop={1}>
            <Text color="gray" dimColor>
              Duration: {formatDuration(toolCall.duration_ms)}
            </Text>
          </Box>
        )}

        {/* Progress indicator for running tools */}
        {toolCall.status === 'running' && toolCall.duration_ms && (
          <Box marginTop={1}>
            <ProgressBar
              progress={Math.min(90, (toolCall.duration_ms / 30000) * 100)} // Fake progress
              width={30}
              color="yellow"
              label="Progress"
            />
          </Box>
        )}
      </Box>
    )
  }

  return (
    <Box flexDirection="column">
      <Box marginBottom={1} gap={2}>
        <Text color="blue" bold>
          Tool Calls
        </Text>
        {!showCompleted && <Text color="gray">(Active only)</Text>}
      </Box>

      {toolCallsArray.map(renderToolCall)}

      {/* Show count if there are more items */}
      {activeToolCalls.size > maxItems && (
        <Box marginTop={1}>
          <Text color="gray" dimColor>
            ... and {activeToolCalls.size - maxItems} more tool calls
          </Text>
        </Box>
      )}
    </Box>
  )
}

// Component for displaying a single tool call inline
export interface InlineToolCallProps {
  toolCall: ToolCall
  compact?: boolean
}

export const InlineToolCall: React.FC<InlineToolCallProps> = ({
  toolCall,
  compact = false,
}) => {
  const statusColor = STATUS_COLORS[toolCall.status]
  const statusIcon = STATUS_ICONS[toolCall.status]

  if (compact) {
    return (
      <Box>
        <Text>{statusIcon} </Text>
        <Text color={statusColor}>{toolCall.name}</Text>
        {toolCall.status === 'running' && <Spinner type="dots" />}
      </Box>
    )
  }

  return (
    <Box
      flexDirection="column"
      borderStyle="single"
      borderColor={statusColor}
      padding={1}
      marginTop={1}
    >
      <Box>
        <Text>{statusIcon} </Text>
        <Text color={statusColor} bold>
          {toolCall.name}
        </Text>
        {toolCall.status === 'running' && <Spinner type="dots" />}
      </Box>

      {toolCall.error && (
        <Box marginTop={1}>
          <Text color="red">{toolCall.error}</Text>
        </Box>
      )}

      {toolCall.duration_ms && (
        <Box marginTop={1}>
          <Text color="gray" dimColor>
            {formatDuration(toolCall.duration_ms)}
          </Text>
        </Box>
      )}
    </Box>
  )
}
