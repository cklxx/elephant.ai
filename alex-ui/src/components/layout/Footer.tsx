import React from 'react'
import { Box, Text } from 'ink'
import { useUIStore } from '@/stores'

export interface FooterProps {
  showShortcuts?: boolean
  showVersion?: boolean
}

const SHORTCUTS = [
  { key: 'Ctrl+C', description: 'Exit' },
  { key: 'Ctrl+L', description: 'Clear' },
  { key: 'Ctrl+N', description: 'New Session' },
  { key: 'Ctrl+S', description: 'Save Session' },
  { key: '↑↓', description: 'History' },
]

export const Footer: React.FC<FooterProps> = ({
  showShortcuts = true,
  showVersion = true,
}) => {
  const { errorState } = useUIStore()

  return (
    <Box
      flexDirection="column"
      padding={1}
      borderStyle="round"
      borderColor="gray"
    >
      {/* Error display */}
      {errorState && (
        <Box marginBottom={1} padding={1} borderStyle="single" borderColor="red" gap={1}>
          <Text color="red" bold>
            Error:
          </Text>
          <Text color="red">
            {errorState.message}
          </Text>
          {errorState.recoverable && (
            <Text color="yellow">
              (Press Enter to retry)
            </Text>
          )}
        </Box>
      )}

      {/* Shortcuts and version */}
      <Box justifyContent="space-between">
        {/* Keyboard shortcuts */}
        {showShortcuts && (
          <Box>
            {SHORTCUTS.map((shortcut) => (
              <Box key={shortcut.key} marginRight={3} gap={1}>
                <Text color="blue">{shortcut.key}</Text>
                <Text color="gray">:</Text>
                <Text color="gray">
                  {shortcut.description}
                </Text>
              </Box>
            ))}
          </Box>
        )}

        {/* Version */}
        {showVersion && (
          <Box>
            <Text color="gray" dimColor>
              ALEX UI v0.1.0
            </Text>
          </Box>
        )}
      </Box>
    </Box>
  )
}