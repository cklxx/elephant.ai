import React from 'react'
import { Box, Text } from 'ink'
import { Message, MessageContent } from '@/types'
import { useUIStore } from '@/stores'
import { formatTimestamp } from '@/utils/formatting'
import {
  formatCodeBlock,
  extractCodeBlocks,
  isCodeLikeContent,
} from '@/utils/syntax-highlighting'

export interface MessageItemProps {
  message: Message
  showTimestamp?: boolean
  showMetadata?: boolean
  maxWidth?: number
}

export const MessageItem: React.FC<MessageItemProps> = ({
  message,
  showTimestamp,
  showMetadata,
}) => {
  const { enableSyntaxHighlighting } = useUIStore()

  const formatContent = React.useCallback(
    (content: MessageContent) => {
      switch (content.type) {
        case 'text':
          if (!content.text) return null

          // Check if content looks like code
          if (enableSyntaxHighlighting && isCodeLikeContent(content.text)) {
            const codeBlocks = extractCodeBlocks(content.text)

            if (codeBlocks.length > 0) {
              // Render with syntax highlighting
              let result = content.text
              codeBlocks.forEach(block => {
                const highlighted = formatCodeBlock(block.code, block.language)
                result = result.replace(block.code, highlighted)
              })
              return result
            }
          }

          return content.text

        case 'tool_call':
          return `🔧 Tool Call: ${content.tool_name}`

        case 'tool_result':
          return `✅ Tool Result: ${content.tool_result ? 'Success' : 'Failed'}`

        default:
          return 'Unknown content type'
      }
    },
    [enableSyntaxHighlighting]
  )

  const renderContentItem = (content: MessageContent, index: number) => {
    const formattedContent = formatContent(content)
    if (!formattedContent) return null

    return (
      <Box key={index} marginTop={index > 0 ? 1 : 0}>
        {content.type === 'tool_call' && (
          <Box
            flexDirection="column"
            borderStyle="round"
            borderColor="yellow"
            padding={1}
          >
            <Text color="yellow" bold>
              🔧 Tool Call: {content.tool_name}
            </Text>
            {content.tool_input && (
              <Box marginTop={1}>
                <Text color="gray">
                  Input: {JSON.stringify(content.tool_input, null, 2)}
                </Text>
              </Box>
            )}
          </Box>
        )}

        {content.type === 'tool_result' && (
          <Box
            flexDirection="column"
            borderStyle="round"
            borderColor="green"
            padding={1}
          >
            <Text color="green" bold>
              ✅ Tool Result
            </Text>
            {content.tool_result && (
              <Box marginTop={1}>
                <Text>
                  {typeof content.tool_result === 'string'
                    ? content.tool_result
                    : JSON.stringify(content.tool_result, null, 2)}
                </Text>
              </Box>
            )}
          </Box>
        )}

        {content.type === 'text' && (
          <Box>
            <Text>{formattedContent}</Text>
          </Box>
        )}
      </Box>
    )
  }

  return (
    <Box flexDirection="column" marginBottom={1}>
      {/* Message header */}
      <Box>
        <Text bold color="blue">
          {message.role.toUpperCase()}
        </Text>
        {showTimestamp && (
          <Box marginLeft={2}>
            <Text color="gray">{formatTimestamp(message.timestamp)}</Text>
          </Box>
        )}
      </Box>

      {/* Message content */}
      <Box flexDirection="column" marginLeft={2} marginTop={1}>
        {message.content.map((content, index) =>
          renderContentItem(content, index)
        )}
      </Box>

      {/* Message metadata */}
      {showMetadata && message.metadata && (
        <Box marginTop={1} marginLeft={2}>
          <Text color="gray" dimColor>
            Metadata: {JSON.stringify(message.metadata)}
          </Text>
        </Box>
      )}
    </Box>
  )
}
