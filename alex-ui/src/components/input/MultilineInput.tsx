import React from 'react'
import { Box, Text, useInput } from 'ink'
import { useUIStore } from '@/stores'
import { wrapText } from '@/utils/formatting'

export interface MultilineInputProps {
  width?: number
  maxLines?: number
  onSubmit?: (value: string) => void
  onCancel?: () => void
}

export const MultilineInput: React.FC<MultilineInputProps> = ({
  width = 80,
  maxLines = 10,
  onSubmit,
  onCancel,
}) => {
  const { inputState, updateInputState } = useUIStore()

  const lines = React.useMemo(() => {
    return wrapText(inputState.value, width - 4) // Account for prefix
  }, [inputState.value, width])

  const visibleLines = React.useMemo(() => {
    return lines.slice(Math.max(0, lines.length - maxLines))
  }, [lines, maxLines])

  useInput((input, key) => {
    if (key.ctrl && (input === 'd' || input === 'c')) {
      // Cancel multiline input
      onCancel?.()
      return
    }

    if (key.ctrl && input === 'm') {
      // Submit with Ctrl+M
      onSubmit?.(inputState.value)
      return
    }

    if (key.return) {
      // Add new line
      const newValue = inputState.value + '\n'
      updateInputState({
        value: newValue,
        cursorPosition: newValue.length,
      })
      return
    }

    if (key.backspace || key.delete) {
      if (inputState.cursorPosition > 0) {
        const newValue =
          inputState.value.slice(0, inputState.cursorPosition - 1) +
          inputState.value.slice(inputState.cursorPosition)

        updateInputState({
          value: newValue,
          cursorPosition: inputState.cursorPosition - 1,
        })
      }
      return
    }

    // Handle regular input
    if (input && !key.ctrl && !key.meta) {
      const newValue =
        inputState.value.slice(0, inputState.cursorPosition) +
        input +
        inputState.value.slice(inputState.cursorPosition)

      updateInputState({
        value: newValue,
        cursorPosition: inputState.cursorPosition + input.length,
      })
    }
  })

  return (
    <Box flexDirection="column" borderStyle="round" borderColor="blue" padding={1}>
      <Box marginBottom={1}>
        <Text color="blue" bold>
          Multiline Input Mode
        </Text>
        <Text color="gray"> (Ctrl+M to submit, Ctrl+C to cancel)</Text>
      </Box>

      {visibleLines.map((line, index) => (
        <Box key={index}>
          <Text color="blue">{'  '}</Text>
          <Text>{line}</Text>
        </Box>
      ))}

      <Box marginTop={1}>
        <Text color="gray">
          Lines: {lines.length} | Characters: {inputState.value.length}
        </Text>
      </Box>
    </Box>
  )
}