import React from 'react'
import { Box, Text } from 'ink'

export interface SpinnerProps {
  label?: string
  type?: 'dots' | 'line' | 'bounce' | 'pulse'
}

const SPINNER_FRAMES = {
  dots: ['⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'],
  line: ['|', '/', '-', '\\'],
  bounce: ['⠁', '⠂', '⠄', '⠂'],
  pulse: ['●', '○', '●', '○'],
}

export const Spinner: React.FC<SpinnerProps> = ({ label, type = 'dots' }) => {
  const [frameIndex, setFrameIndex] = React.useState(0)
  const frames = SPINNER_FRAMES[type]

  React.useEffect(() => {
    const interval = setInterval(() => {
      setFrameIndex(current => (current + 1) % frames.length)
    }, 80)

    return () => clearInterval(interval)
  }, [frames.length])

  return (
    <Box>
      <Text color="blue">{frames[frameIndex]}</Text>
      {label && <Text> {label}</Text>}
    </Box>
  )
}
