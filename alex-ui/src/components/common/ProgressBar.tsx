import React from 'react'
import { Box, Text } from 'ink'

export interface ProgressBarProps {
  progress: number // 0-100
  width?: number
  showPercentage?: boolean
  label?: string
  color?: 'green' | 'blue' | 'yellow' | 'red' | 'cyan' | 'magenta'
}

export const ProgressBar: React.FC<ProgressBarProps> = ({
  progress,
  width = 20,
  showPercentage = true,
  label,
  color = 'blue',
}) => {
  const clampedProgress = Math.max(0, Math.min(100, progress))
  const filled = Math.floor((clampedProgress / 100) * width)
  const empty = width - filled

  const filledBar = '█'.repeat(filled)
  const emptyBar = '░'.repeat(empty)

  return (
    <Box>
      {label && <Text>{label} </Text>}
      <Text color={color}>[{filledBar}</Text>
      <Text color="gray">{emptyBar}</Text>
      <Text color={color}>]</Text>
      {showPercentage && <Text> {clampedProgress.toFixed(1)}%</Text>}
    </Box>
  )
}
