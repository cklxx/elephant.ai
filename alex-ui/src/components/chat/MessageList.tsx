import React from 'react'
import { Box, Text } from 'ink'
import { useMessageStore, useUIStore } from '@/stores'
import { MessageItem } from './MessageItem'
import { StreamingIndicator } from './StreamingIndicator'

export interface MessageListProps {
  maxHeight?: number
  maxWidth?: number
  autoScroll?: boolean
}

export const MessageList: React.FC<MessageListProps> = ({
  maxHeight = 20,
  maxWidth = 80,
  autoScroll = true,
}) => {
  const { messageThread, streamingState } = useMessageStore()
  const { showTimestamps, showMetadata } = useUIStore()

  const scrollRef = React.useRef<HTMLDivElement>(null)

  // Auto-scroll to bottom when new messages arrive
  React.useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messageThread.messages, autoScroll])

  const displayMessages = React.useMemo(() => {
    // Show the most recent messages that fit in the display area
    return messageThread.messages.slice(-maxHeight)
  }, [messageThread.messages, maxHeight])

  if (displayMessages.length === 0) {
    return (
      <Box
        flexDirection="column"
        justifyContent="center"
        alignItems="center"
        height={maxHeight}
        borderStyle="round"
        borderColor="gray"
      >
        <Text color="gray">No messages yet</Text>
        <Text color="gray" dimColor>
          Start a conversation by typing a message below
        </Text>
      </Box>
    )
  }

  return (
    <Box
      flexDirection="column"
      height={maxHeight}
      borderStyle="round"
      borderColor="blue"
      padding={1}
    >
      {/* Message history indicator */}
      {messageThread.messages.length > maxHeight && (
        <Box marginBottom={1}>
          <Text color="gray" dimColor>
            ... {messageThread.messages.length - maxHeight} earlier messages
          </Text>
        </Box>
      )}

      {/* Messages */}
      <Box flexDirection="column" flexGrow={1}>
        {displayMessages.map(message => (
          <MessageItem
            key={message.id}
            message={message}
            showTimestamp={showTimestamps}
            showMetadata={showMetadata}
            maxWidth={maxWidth - 4} // Account for padding
          />
        ))}
      </Box>

      {/* Streaming indicator */}
      {streamingState.isActive && (
        <Box marginTop={1}>
          <StreamingIndicator
            phase={streamingState.phase}
            progress={streamingState.progress}
            estimatedTimeMs={streamingState.estimatedTimeMs}
          />
        </Box>
      )}

      {/* Current streaming message */}
      {messageThread.currentStreamingMessage && (
        <Box marginTop={1} borderStyle="single" borderColor="yellow" padding={1}>
          <Text color="yellow" bold>
            Streaming...
          </Text>
          <MessageItem
            message={messageThread.currentStreamingMessage as any}
            showTimestamp={false}
            showMetadata={false}
            maxWidth={maxWidth - 6} // Account for border and padding
          />
        </Box>
      )}
    </Box>
  )
}