import React from 'react'
import { Box, Text, useInput } from 'ink'
import { useUIStore, useMessageStore } from '@/stores'
import { KeyboardHandler, parseKeyPress } from '@/utils/keyboard'
import { validateMessageContent, sanitizeInput } from '@/utils/validation'

export interface CommandInputProps {
  placeholder?: string
  onSubmit?: (value: string) => void
  disabled?: boolean
}

export const CommandInput: React.FC<CommandInputProps> = ({
  placeholder = 'Type your message...',
  onSubmit,
  disabled = false,
}) => {
  const {
    inputState,
    updateInputState,
    setError,
    clearError,
  } = useUIStore()

  const { sendMessage, streamingState } = useMessageStore()

  const keyboardHandler = React.useMemo(() => new KeyboardHandler(), [])

  const handleSubmit = React.useCallback(async () => {
    const trimmedValue = inputState.value.trim()

    if (!trimmedValue) {
      return
    }

    if (!validateMessageContent(trimmedValue)) {
      setError({
        message: 'Invalid message content',
        recoverable: true,
        timestamp: new Date().toISOString(),
      })
      return
    }

    try {
      clearError()

      // Add to history
      const newHistory = [trimmedValue, ...inputState.history.slice(0, 99)]
      updateInputState({
        history: newHistory,
        historyIndex: -1,
        value: '',
        cursorPosition: 0,
      })

      // Send message
      if (onSubmit) {
        onSubmit(trimmedValue)
      } else {
        await sendMessage(trimmedValue)
      }
    } catch (error) {
      setError({
        message: error instanceof Error ? error.message : 'Failed to send message',
        recoverable: true,
        timestamp: new Date().toISOString(),
      })
    }
  }, [inputState, onSubmit, sendMessage, setError, clearError, updateInputState])

  const handleHistoryNavigation = React.useCallback((direction: 'up' | 'down') => {
    const { history, historyIndex } = inputState

    if (history.length === 0) return

    let newIndex: number
    if (direction === 'up') {
      newIndex = historyIndex < history.length - 1 ? historyIndex + 1 : historyIndex
    } else {
      newIndex = historyIndex > -1 ? historyIndex - 1 : -1
    }

    const newValue = newIndex === -1 ? '' : history[newIndex]

    updateInputState({
      historyIndex: newIndex,
      value: newValue,
      cursorPosition: newValue.length,
    })
  }, [inputState, updateInputState])

  useInput((input, key) => {
    if (disabled || streamingState.isActive) {
      return
    }

    const keyPress = parseKeyPress(input)

    // Handle special keys
    if (key.return) {
      handleSubmit()
      return
    }

    if (key.upArrow) {
      handleHistoryNavigation('up')
      return
    }

    if (key.downArrow) {
      handleHistoryNavigation('down')
      return
    }

    if (key.leftArrow) {
      const newPosition = Math.max(0, inputState.cursorPosition - 1)
      updateInputState({ cursorPosition: newPosition })
      return
    }

    if (key.rightArrow) {
      const newPosition = Math.min(inputState.value.length, inputState.cursorPosition + 1)
      updateInputState({ cursorPosition: newPosition })
      return
    }

    if (key.backspace || key.delete) {
      if (inputState.cursorPosition > 0) {
        const newValue =
          inputState.value.slice(0, inputState.cursorPosition - 1) +
          inputState.value.slice(inputState.cursorPosition)
        const newPosition = inputState.cursorPosition - 1

        updateInputState({
          value: newValue,
          cursorPosition: newPosition,
          historyIndex: -1,
        })
      }
      return
    }

    if (key.ctrl && input === 'u') {
      // Clear line
      updateInputState({
        value: '',
        cursorPosition: 0,
        historyIndex: -1,
      })
      return
    }

    if (key.ctrl && input === 'a') {
      // Move to beginning
      updateInputState({ cursorPosition: 0 })
      return
    }

    if (key.ctrl && input === 'e') {
      // Move to end
      updateInputState({ cursorPosition: inputState.value.length })
      return
    }

    // Handle keyboard shortcuts
    if (keyboardHandler.handleKeyPress(keyPress)) {
      return
    }

    // Handle regular character input
    if (input && !key.ctrl && !key.meta) {
      const sanitized = sanitizeInput(input)
      if (sanitized) {
        const newValue =
          inputState.value.slice(0, inputState.cursorPosition) +
          sanitized +
          inputState.value.slice(inputState.cursorPosition)

        updateInputState({
          value: newValue,
          cursorPosition: inputState.cursorPosition + sanitized.length,
          historyIndex: -1,
        })
      }
    }
  })

  const renderInput = () => {
    const { value, cursorPosition } = inputState
    const beforeCursor = value.slice(0, cursorPosition)
    const atCursor = value[cursorPosition] || ' '
    const afterCursor = value.slice(cursorPosition + 1)

    if (disabled || streamingState.isActive) {
      return (
        <Box>
          <Text color="gray">{placeholder}</Text>
        </Box>
      )
    }

    return (
      <Box>
        <Text color="blue">{'> '}</Text>
        <Text>{beforeCursor}</Text>
        <Text backgroundColor="white" color="black">
          {atCursor}
        </Text>
        <Text>{afterCursor}</Text>
      </Box>
    )
  }

  return (
    <Box flexDirection="column">
      {renderInput()}
      {streamingState.isActive && (
        <Box marginTop={1}>
          <Text color="yellow">
            {streamingState.phase === 'thinking' && 'Thinking...'}
            {streamingState.phase === 'acting' && 'Acting...'}
            {streamingState.phase === 'observing' && 'Observing...'}
            {streamingState.phase === 'responding' && 'Responding...'}
          </Text>
        </Box>
      )}
    </Box>
  )
}