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
	tokenEstimator *TokenEstimator
}

// NewMessageCompressor creates a new message compressor
func NewMessageCompressor(sessionManager *session.Manager, llmClient llm.Client) *MessageCompressor {
	return &MessageCompressor{
		sessionManager: sessionManager,
		llmClient:      llmClient,
		tokenEstimator: NewTokenEstimator(),
	}
}

// CompressMessages compresses messages using cache-friendly strategy
// Keeps stable prefix for context caching, compresses middle, preserves recent active
func (mc *MessageCompressor) CompressMessages(ctx context.Context, messages []*session.Message, actualTokens ...int) []*session.Message {
	totalTokens := mc.estimateTokens(messages)
	messageCount := len(messages)

	// Use actual token count if provided (more accurate than estimation)
	if len(actualTokens) > 0 && actualTokens[0] > 0 {
		totalTokens = actualTokens[0]
		log.Printf("[DEBUG] Using actual token count: %d messages, %d actual tokens", messageCount, totalTokens)
	} else {
		log.Printf("[DEBUG] Token estimation: %d messages, %d estimated tokens", messageCount, totalTokens)
	}

	// Simplified compression thresholds
	const (
		TokenThreshold   = 100000 // 100K token limit as requested
		MessageThreshold = 15     // Lower threshold for earlier compression
	)

	// Only compress if we exceed thresholds significantly
	if messageCount > MessageThreshold && totalTokens > TokenThreshold {
		log.Printf("[INFO] Simplified compression triggered: %d messages, %d tokens", messageCount, totalTokens)
		return mc.simplifiedCompress(ctx, messages)
	}

	log.Printf("[DEBUG] Compression skipped: %d messages (%d threshold), %d tokens (%d threshold)",
		messageCount, MessageThreshold, totalTokens, TokenThreshold)

	return messages
}

// simplifiedCompress implements simplified compression strategy
// Keeps only system messages, compresses all others
func (mc *MessageCompressor) simplifiedCompress(ctx context.Context, messages []*session.Message) []*session.Message {
	// Step 1: 分离系统消息和非系统消息
	var systemMessages []*session.Message
	var nonSystemMessages []*session.Message

	for _, msg := range messages {
		if msg.Role == "system" {
			systemMessages = append(systemMessages, msg)
		} else {
			nonSystemMessages = append(nonSystemMessages, msg)
		}
	}

	if len(nonSystemMessages) == 0 {
		return messages // 只有系统消息，不需要压缩
	}

	log.Printf("[DEBUG] Simplified compression: system=%d, non-system=%d",
		len(systemMessages), len(nonSystemMessages))

	// Step 2: 压缩全部非系统消息
	compressedRemaining := mc.compressRemainingMessages(ctx, nonSystemMessages)

	// Step 3: 重新组合消息
	result := make([]*session.Message, 0, len(systemMessages)+1)

	// 添加系统消息
	result = append(result, systemMessages...)
	// 添加压缩的非系统消息
	if compressedRemaining != nil {
		result = append(result, compressedRemaining)
	}

	log.Printf("[INFO] Simplified compression completed: %d -> %d messages", len(messages), len(result))
	return result
}

// compressRemainingMessages compresses remaining messages using AI summarization
func (mc *MessageCompressor) compressRemainingMessages(ctx context.Context, messages []*session.Message) *session.Message {
	if len(messages) == 0 {
		return nil
	}

	// 使用现有的 AI 摘要方法
	summaryMsg := mc.createComprehensiveAISummary(ctx, messages)
	if summaryMsg == nil {
		return nil
	}

	// 标记为简化压缩
	if summaryMsg.Metadata == nil {
		summaryMsg.Metadata = make(map[string]any)
	}
	summaryMsg.Metadata["simplified_compression"] = true
	summaryMsg.Metadata["original_message_count"] = len(messages)

	log.Printf("[DEBUG] Compressed %d remaining messages into summary", len(messages))
	return summaryMsg
}

// createComprehensiveAISummary creates a comprehensive AI summary preserving important context
func (mc *MessageCompressor) createComprehensiveAISummary(ctx context.Context, messages []*session.Message) *session.Message {
	if mc.llmClient == nil || len(messages) == 0 {
		return mc.createStatisticalSummary(messages)
	}

	conversationText := mc.buildComprehensiveSummaryInput(messages)
	prompt := mc.buildComprehensiveSummaryPrompt(conversationText, len(messages))

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
		ModelType: llm.BasicModel,
		Config: &llm.Config{
			Temperature: 0.1,  // Even lower temperature for more consistent structured output
			MaxTokens:   2000, // More tokens for comprehensive structured summaries
		},
	}

	// Use the provided context with timeout to preserve session ID and other values
	timeoutCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	sessionID, _ := mc.sessionManager.GetSessionID()
	response, err := mc.llmClient.Chat(timeoutCtx, request, sessionID)
	if err != nil {
		log.Printf("[WARN] MessageCompressor: Comprehensive AI summary failed: %v", err)
		return mc.createStatisticalSummary(messages)
	}

	if len(response.Choices) == 0 {
		return mc.createStatisticalSummary(messages)
	}

	return &session.Message{
		Role:    "system",
		Content: fmt.Sprintf("Comprehensive conversation summary (%d messages): %s", len(messages), response.Choices[0].Message.Content),
		Metadata: map[string]any{
			"type":           "comprehensive_ai_summary",
			"original_count": len(messages),
			"created_at":     time.Now().Unix(),
			"summary_method": "ai_comprehensive",
		},
		Timestamp: time.Now(),
	}
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
func (mc *MessageCompressor) buildComprehensiveSummaryInput(messages []*session.Message) string {
	var parts []string

	for i, msg := range messages {
		if msg.Role != "system" && len(strings.TrimSpace(msg.Content)) > 0 {
			// Include message index for context
			content := msg.Content

			// Include tool call information if present
			if len(msg.ToolCalls) > 0 {
				var toolInfo []string
				for _, tc := range msg.ToolCalls {
					toolInfo = append(toolInfo, fmt.Sprintf("Tool: %s", tc.Name))
				}
				content += fmt.Sprintf(" [Tool calls: %s]", strings.Join(toolInfo, ", "))
			}

			// Include tool response metadata if present
			if msg.Role == "tool" {
				if toolName, ok := msg.Metadata["tool_name"].(string); ok {
					content = fmt.Sprintf("[%s result]: %s", toolName, content)
				}
			}

			parts = append(parts, fmt.Sprintf("[Message %d - %s]: %s", i+1, msg.Role, content))
		}
	}

	text := strings.Join(parts, "\n\n")

	return text
}

// createStatisticalSummary creates a summary based on statistics
func (mc *MessageCompressor) createStatisticalSummary(messages []*session.Message) *session.Message {
	userCount := 0
	assistantCount := 0
	toolCount := 0

	for _, msg := range messages {
		switch msg.Role {
		case "user":
			userCount++
		case "assistant":
			assistantCount++
		case "tool":
			toolCount++
		}
	}

	summary := fmt.Sprintf("Previous conversation summary: %d messages (%d user, %d assistant, %d tool)",
		len(messages), userCount, assistantCount, toolCount)

	return &session.Message{
		Role:    "system",
		Content: summary,
		Metadata: map[string]any{
			"type":            "statistical_summary",
			"original_count":  len(messages),
			"user_count":      userCount,
			"assistant_count": assistantCount,
			"tool_count":      toolCount,
			"created_at":      time.Now().Unix(),
		},
		Timestamp: time.Now(),
	}
}

// estimateTokens estimates the total tokens in messages with improved accuracy
func (mc *MessageCompressor) estimateTokens(messages []*session.Message) int {
	total := 0
	for _, msg := range messages {
		// Use more accurate token estimation
		contentTokens := mc.estimateContentTokens(msg.Content)

		// Add overhead for role, metadata, and structure
		roleTokens := 5                          // role field
		metadataTokens := len(msg.Metadata) * 10 // rough metadata overhead

		// Add tokens for tool calls
		toolCallTokens := 0
		for _, tc := range msg.ToolCalls {
			toolCallTokens += len(tc.Name)/3 + len(tc.ID)/3 + 15 // Tool call structure overhead
		}

		messageTotal := contentTokens + roleTokens + metadataTokens + toolCallTokens
		total += messageTotal

		// Debug individual message token count for large messages
		if contentTokens > 1000 {
			log.Printf("[DEBUG] Large message: %d content tokens, %d total tokens (role: %d, metadata: %d, tools: %d)",
				contentTokens, messageTotal, roleTokens, metadataTokens, toolCallTokens)
		}
	}
	return total
}

// estimateContentTokens provides more accurate token estimation for message content
func (mc *MessageCompressor) estimateContentTokens(content string) int {
	if content == "" {
		return 0
	}

	// More sophisticated estimation based on content type
	length := len(content)

	// Different ratios for different content types
	var charsPerToken = 4.0 // Default for regular text

	// Adjust for code-heavy content (more token-dense)
	if strings.Contains(content, "```") || strings.Contains(content, "func ") || strings.Contains(content, "import ") {
		charsPerToken = 2.5 // Code is more token-dense
	}

	// Adjust for JSON/structured data
	if strings.Contains(content, `"role"`) || strings.Contains(content, `{"`) {
		charsPerToken = 3.0 // JSON is moderately token-dense
	}

	// Adjust for very long content (usually less token-dense due to repetition)
	if length > 10000 {
		charsPerToken = 5.0
	}

	estimated := int(float64(length) / charsPerToken)

	// Minimum of 1 token for non-empty content
	if estimated == 0 && length > 0 {
		estimated = 1
	}

	return estimated
}
