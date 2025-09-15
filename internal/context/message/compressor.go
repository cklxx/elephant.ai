package message

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"alex/internal/llm"
	"alex/internal/session"
)

// MessageCompressor handles message compression operations
type MessageCompressor struct {
	sessionManager *session.Manager
	llmClient      llm.Client
	llmConfig      *llm.Config
}

// NewMessageCompressor creates a new message compressor
func NewMessageCompressor(sessionManager *session.Manager, llmClient llm.Client, llmConfig *llm.Config) *MessageCompressor {
	mc := &MessageCompressor{
		sessionManager: sessionManager,
		llmClient:      llmClient,
		llmConfig:      llmConfig,
	}

	return mc
}

// CompressMessages compresses messages using cache-friendly strategy with async support
// Keeps stable prefix for context caching, compresses middle, preserves recent active
// consumedTokens: total tokens consumed in session (accumulative)
// currentTokens: current message tokens from result.PromptTokens (reset to zero after compression)
func (mc *MessageCompressor) CompressMessages(ctx context.Context, messages []llm.Message, consumedTokens int, currentTokens int) ([]llm.Message, int, int) {
	messageCount := len(messages)

	// Compression thresholds
	const (
		TokenThreshold   = 100000 // 100K token limit
		MessageThreshold = 15     // Message count threshold
	)

	// Only compress if we exceed thresholds significantly
	if messageCount > MessageThreshold && currentTokens > TokenThreshold {
		log.Printf("[INFO] AI compression triggered: %d messages, %d current tokens", messageCount, currentTokens)

		compressedMessages := mc.compressWithAI(ctx, messages)
		
		// Update session with compressed messages
		mc.updateSessionWithCompression(messages, compressedMessages)

		// Update token counters after compression
		newConsumedTokens := consumedTokens + currentTokens // Add current to consumed
		newCurrentTokens := 0                               // Reset current tokens to zero after compression

		log.Printf("[INFO] Token tracking after compression: consumed=%d->%d, current=%d->%d",
			consumedTokens, newConsumedTokens, currentTokens, newCurrentTokens)

		return compressedMessages, newConsumedTokens, newCurrentTokens
	}

	return messages, consumedTokens, currentTokens
}

// compressWithAI implements AI-based compression strategy
// Keeps only system messages, compresses all others using AI
func (mc *MessageCompressor) compressWithAI(ctx context.Context, messages []llm.Message) []llm.Message {
	// Step 1: 分离系统消息和非系统消息
	var systemMessages []llm.Message
	var nonSystemMessages []llm.Message

	if len(messages) <= 2 {
		return messages
	}

	systemMessages = messages[:2]
	nonSystemMessages = messages[2:]

	// Separating system and non-system messages for compression

	// Step 2: 使用AI压缩全部非系统消息，保留历史消息
	compressedMessage := mc.createComprehensiveAISummaryWithHistory(ctx, nonSystemMessages)

	// Step 3: 重新组合消息
	result := make([]llm.Message, 0, len(systemMessages)+1)

	// 添加系统消息
	result = append(result, systemMessages...)
	// 添加压缩的非系统消息
	if compressedMessage != nil {
		result = append(result, *compressedMessage)
	}

	log.Printf("[INFO] AI compression completed: %d -> %d messages", len(messages), len(result))
	return result
}

// createComprehensiveAISummaryWithHistory creates a comprehensive AI summary preserving original messages
func (mc *MessageCompressor) createComprehensiveAISummaryWithHistory(ctx context.Context, messages []llm.Message) *llm.Message {
	if mc.llmClient == nil {
		return nil
	}
	if len(messages) == 0 {
		return nil
	}

	return mc.createComprehensiveAISummaryWithRetry(ctx, messages)
}

// createComprehensiveAISummaryWithRetry implements retry logic with progressive message deletion for timeout and limit errors
func (mc *MessageCompressor) createComprehensiveAISummaryWithRetry(ctx context.Context, messages []llm.Message) *llm.Message {
	currentMessages := make([]llm.Message, len(messages))
	copy(currentMessages, messages)
	originalMessageCount := len(messages)

	// Minimum number of messages to keep for meaningful compression
	const minMessages = 3

	for len(currentMessages) >= minMessages {
		conversationText := mc.buildComprehensiveSummaryInput(currentMessages)
		prompt := mc.buildComprehensiveSummaryPrompt(conversationText, len(currentMessages))

		request := &llm.ChatRequest{
			Messages: []llm.Message{
				{
					Role:    "system",
					Content: mc.buildComprehensiveSystemPrompt(),
				},
				{
					Role:    "user",
					Content: prompt,
				},
			},
			ModelType: mc.llmConfig.DefaultModelType,
			Config:    mc.llmConfig,
		}

		// Use shorter timeout to prevent blocking, with fallback handling
		timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		sessionID, _ := mc.sessionManager.GetSessionID()
		response, err := mc.llmClient.Chat(timeoutCtx, request, sessionID)
		cancel()

		if err != nil {
			// Check if it's a timeout or context limit error
			if isTimeoutOrLimitError(err) {
				// Delete the last message and retry
				if len(currentMessages) > minMessages {
					currentMessages = currentMessages[:len(currentMessages)-1]
					log.Printf("[WARN] MessageCompressor: %v, removing 1 message (%d->%d) and retrying", err, len(currentMessages)+1, len(currentMessages))
					continue
				} else {
					log.Printf("[ERROR] MessageCompressor: Cannot reduce messages further, minimum reached. Original error: %v", err)
					return nil
				}
			} else {
				// Other errors, fail immediately
				log.Printf("[ERROR] MessageCompressor: Comprehensive AI summary failed: %v", err)
				return nil
			}
		}

		if len(response.Choices) == 0 {
			log.Printf("[ERROR] MessageCompressor: No response choices from AI summary")
			return nil
		}

		// Create compressed message with history preservation
		compressedContent := fmt.Sprintf("Comprehensive conversation summary (%d->%d messages): %s", originalMessageCount, len(currentMessages), response.Choices[0].Message.Content)

		// Create compressed message with source history using original messages
		compressedMsg := llm.NewCompressedMessage("user", compressedContent, messages)

		if len(currentMessages) < originalMessageCount {
			log.Printf("[INFO] AI compression completed with %d message(s) removed due to limits", originalMessageCount-len(currentMessages))
		} else {
			log.Printf("[INFO] AI compression completed successfully with history preservation")
		}
		return compressedMsg
	}

	log.Printf("[ERROR] MessageCompressor: Unable to compress - too few messages remaining")
	return nil
}


// buildComprehensiveSystemPrompt builds the system prompt for comprehensive AI summarization
func (mc *MessageCompressor) buildComprehensiveSystemPrompt() string {
	return `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and the assistant's previous actions.

This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:

1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - The assistant's approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like:
     - file names
     - full code snippets
     - function signatures
     - file edits
   - Errors that were encountered and how they were fixed
   - Pay special attention to specific user feedback that was received, especially if the user told the assistant to do something differently.

2. Double-check for technical accuracy and completeness, addressing each required element thoroughly.

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Pay special attention to the most recent messages and include full code snippets where applicable and include a summary of why this file read or edit is important.
4. Errors and fixes: List all errors that were encountered, and how they were fixed. Pay special attention to specific user feedback that was received, especially if the user told the assistant to do something differently.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results. These are critical for understanding the users' feedback and changing intent.
7. Pending Tasks: Outline any pending tasks that have been explicitly asked to work on.
8. Current Work: Describe in detail precisely what was being worked on immediately before this summary request, paying special attention to the most recent messages from both user and assistant. Include file names and code snippets where applicable.
9. Optional Next Step: List the next step that will be taken that is related to the most recent work. IMPORTANT: ensure that this step is DIRECTLY in line with the user's explicit requests, and the task that was being worked on immediately before this summary request.

Be concise but comprehensive, focusing on technical accuracy and actionable context.`
}

// buildComprehensiveSummaryPrompt builds the prompt for comprehensive AI summarization
func (mc *MessageCompressor) buildComprehensiveSummaryPrompt(conversationText string, messageCount int) string {
	return fmt.Sprintf(`Please analyze this %d-message conversation and provide a comprehensive summary following the structured format outlined in the system prompt.

CONVERSATION TO ANALYZE:
%s

Please provide your summary using the exact structure specified:

<analysis>
[Your chronological analysis of the conversation, ensuring all points are covered thoroughly and accurately]
</analysis>

<summary>
1. Primary Request and Intent:
   [Detailed description of user's explicit requests and intents]

2. Key Technical Concepts:
   - [Concept 1]
   - [Concept 2]
   - [...]

3. Files and Code Sections:
   - [File Name 1]
     - [Summary of why this file is important]
     - [Summary of the changes made to this file, if any]
     - [Important Code Snippet]
   - [File Name 2]
     - [Important Code Snippet]
   - [...]

4. Errors and fixes:
   - [Detailed description of error 1]:
     - [How the error was fixed]
     - [User feedback on the error if any]
   - [...]

5. Problem Solving:
   [Description of solved problems and ongoing troubleshooting]

6. All user messages:
   - [Detailed non tool use user message]
   - [...]

7. Pending Tasks:
   - [Task 1]
   - [Task 2]
   - [...]

8. Current Work:
   [Precise description of current work]

9. Optional Next Step:
   [Optional Next step to take]
</summary>

Ensure precision and thoroughness in your response, focusing on technical accuracy and actionable context for continuation.`, messageCount, conversationText)
}

// buildComprehensiveSummaryInput builds comprehensive input text for AI summarization
func (mc *MessageCompressor) buildComprehensiveSummaryInput(messages []llm.Message) string {
	var parts []string

	for i, msg := range messages {
		if msg.Role != "system" && len(strings.TrimSpace(msg.Content)) > 0 {
			// Include message index for context
			content := msg.Content

			// Include tool call information if present
			if len(msg.ToolCalls) > 0 {
				var toolInfo []string
				for _, tc := range msg.ToolCalls {
					toolInfo = append(toolInfo, fmt.Sprintf("Tool: %s", tc.Function.Name))
					toolInfo = append(toolInfo, fmt.Sprintf("Arguments: %s", tc.Function.Arguments))
				}
				content += fmt.Sprintf(" [Tool calls: %s]", strings.Join(toolInfo, ", "))
			}

			// Include tool response metadata if present
			if msg.Role == "tool" {
				content = fmt.Sprintf("[%s result]: %s", msg.ToolCallId, content)
			}

			parts = append(parts, fmt.Sprintf("[Message %d - %s]: %s", i+1, msg.Role, content))
		}
	}

	text := strings.Join(parts, "\n\n")

	return text
}

// HandleTokenError handles token limit errors by automatically triggering compression
func (mc *MessageCompressor) HandleTokenError(err error, messages []llm.Message) ([]llm.Message, error) {
	if !isTokenLimitError(err) {
		return messages, err
	}

	log.Printf("[INFO] Token limit exceeded, performing emergency compression")

	// Perform immediate compression
	compressed := mc.compressWithAI(context.Background(), messages)
	if len(compressed) == 0 {
		log.Printf("[WARN] Emergency compression failed, returning original error")
		return messages, err
	}

	log.Printf("[INFO] Emergency compression successful: %d -> %d messages", len(messages), len(compressed))
	return compressed, nil
}

// updateSessionWithCompression updates the session messages after compression
// Replaces the original message range with compressed message containing history
func (mc *MessageCompressor) updateSessionWithCompression(originalMessages, compressedMessages []llm.Message) {
	if mc.sessionManager == nil {
		log.Printf("[WARN] SessionManager is nil, cannot update session with compression")
		return
	}

	_, exists := mc.sessionManager.GetSessionID()
	if !exists {
		log.Printf("[WARN] Cannot get session ID for compression update: no active session")
		return
	}

	// Get current session
	session, err := mc.sessionManager.GetCurrentSession()
	if err != nil {
		log.Printf("[WARN] Cannot get session for compression update: %v", err)
		return
	}

	// Find compressed message in the result
	var compressedMsg *llm.Message
	for i := range compressedMessages {
		if compressedMessages[i].IsCompressed {
			compressedMsg = &compressedMessages[i]
			break
		}
	}

	if compressedMsg == nil {
		log.Printf("[WARN] No compressed message found in compression result")
		return
	}

	// Convert to session message format
	sessionMsg := mc.convertLLMToSessionMessage(compressedMsg, originalMessages)
	
	// Update session: replace the non-system message range with compressed message
	// Skip first 2 system messages, compress the rest
	startIdx := 2
	endIdx := len(session.Messages) - 1
	
	if startIdx < len(session.Messages) && endIdx >= startIdx {
		session.ReplaceMessagesWithCompressed(startIdx, endIdx, sessionMsg)
		
		// Save updated session
		if err := mc.sessionManager.SaveSession(session); err != nil {
			log.Printf("[WARN] Failed to save session after compression: %v", err)
		}
	}
	
	log.Printf("[INFO] Session updated with compressed message containing %d source messages", len(originalMessages))
}

// convertLLMToSessionMessage converts LLM message to session message format
func (mc *MessageCompressor) convertLLMToSessionMessage(compressedLLMMsg *llm.Message, originalLLMMessages []llm.Message) *session.Message {
	// Convert LLM messages to session messages for source storage
	sourceMessages := make([]*session.Message, len(originalLLMMessages))
	for i, llmMsg := range originalLLMMessages {
		sourceMessages[i] = &session.Message{
			Role:       llmMsg.Role,
			Content:    llmMsg.Content,
			Name:       llmMsg.Name,
			ToolCalls:  llmMsg.ToolCalls,
			ToolCallId: llmMsg.ToolCallId,
			Timestamp:  time.Now(),
		}
	}
	
	// Create compressed session message
	return &session.Message{
		Role:           compressedLLMMsg.Role,
		Content:        compressedLLMMsg.Content,
		Name:           compressedLLMMsg.Name,
		ToolCalls:      compressedLLMMsg.ToolCalls,
		ToolCallId:     compressedLLMMsg.ToolCallId,
		Metadata:       compressedLLMMsg.Metadata,
		Timestamp:      time.Now(),
		SourceMessages: sourceMessages,
		IsCompressed:   true,
	}
}

// isTokenLimitError checks if the error is related to token limits
func isTokenLimitError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	return (strings.Contains(errStr, "token") &&
		(strings.Contains(errStr, "limit") ||
			strings.Contains(errStr, "exceed") ||
			strings.Contains(errStr, "maximum") ||
			strings.Contains(errStr, "too many") ||
			strings.Contains(errStr, "context length")))
}

// isTimeoutOrLimitError checks if the error is related to timeout or context limits that can be resolved by reducing message count
func isTimeoutOrLimitError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Check for timeout errors
	isTimeout := strings.Contains(errStr, "context deadline exceeded") ||
		strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline")

	// Check for token/context limit errors
	isTokenLimit := strings.Contains(errStr, "token") &&
		(strings.Contains(errStr, "limit") ||
			strings.Contains(errStr, "exceed") ||
			strings.Contains(errStr, "maximum") ||
			strings.Contains(errStr, "too many") ||
			strings.Contains(errStr, "context length") ||
			strings.Contains(errStr, "too long"))

	// Check for request size errors
	isRequestTooLarge := strings.Contains(errStr, "request too large") ||
		strings.Contains(errStr, "payload too large") ||
		strings.Contains(errStr, "content too long")

	return isTimeout || isTokenLimit || isRequestTooLarge
}
