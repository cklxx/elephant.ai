import React from 'react'
import { Box, Text } from 'ink'
import { StreamingState } from '@/types'
import { Spinner, ProgressBar } from '@/components/common'
import { formatDuration } from '@/utils/formatting'

export interface StreamingIndicatorProps {
  phase: StreamingState['phase']
  progress?: number
  estimatedTimeMs?: number
}

const PHASE_DESCRIPTIONS = {
  thinking: 'Analyzing your request and planning the response...',
  acting: 'Executing tools and gathering information...',
  observing: 'Processing tool results and observations...',
  responding: 'Generating the final response...',
} as const

const PHASE_COLORS = {
  thinking: 'blue',
  acting: 'yellow',
  observing: 'cyan',
  responding: 'green',
} as const

export const StreamingIndicator: React.FC<StreamingIndicatorProps> = ({
  phase,
  progress,
  estimatedTimeMs,
}) => {
  const phaseDescription = PHASE_DESCRIPTIONS[phase]
  const phaseColor = PHASE_COLORS[phase]

  return (
    <Box
      flexDirection="column"
      borderStyle="round"
      borderColor={phaseColor}
      padding={1}
    >
      {/* Phase indicator */}
      <Box marginBottom={1} gap={1}>
        <Spinner type="dots" />
        <Text color={phaseColor} bold>
          {phase.toUpperCase()}
        </Text>
      </Box>

      {/* Phase description */}
      <Box marginBottom={progress !== undefined ? 1 : 0}>
        <Text color="gray">{phaseDescription}</Text>
      </Box>

      {/* Progress bar */}
      {progress !== undefined && (
        <Box marginBottom={estimatedTimeMs ? 1 : 0}>
          <ProgressBar
            progress={progress}
            width={40}
            color={phaseColor as 'green' | 'yellow' | 'blue' | 'red'}
            showPercentage={true}
          />
        </Box>
      )}

      {/* Time estimate */}
      {estimatedTimeMs && (
        <Box>
          <Text color="gray" dimColor>
            Estimated time remaining: {formatDuration(estimatedTimeMs)}
          </Text>
        </Box>
      )}
    </Box>
  )
}
