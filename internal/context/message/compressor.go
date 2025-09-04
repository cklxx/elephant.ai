package message

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"alex/internal/llm"
	"alex/internal/session"
)

// MessageCompressor handles message compression operations
type MessageCompressor struct {
	sessionManager *session.Manager
	llmClient      llm.Client
	llmConfig      *llm.Config
	
	// Cache for compression results to avoid redundant calls
	compressionCache map[string]*llm.Message
	cacheMutex       sync.RWMutex
	
	// Channel for async compression results
	compressionJobs  chan compressionJob
	compressionResults map[string]chan compressionResult
	resultsMutex     sync.RWMutex
}

// compressionJob represents an async compression task
type compressionJob struct {
	jobID    string
	context  context.Context
	messages []llm.Message
	resultCh chan compressionResult
}

// compressionResult represents the result of a compression task
type compressionResult struct {
	messages []llm.Message
	err      error
}

// NewMessageCompressor creates a new message compressor
func NewMessageCompressor(sessionManager *session.Manager, llmClient llm.Client, llmConfig *llm.Config) *MessageCompressor {
	mc := &MessageCompressor{
		sessionManager: sessionManager,
		llmClient:      llmClient,
		llmConfig:      llmConfig,
		compressionCache: make(map[string]*llm.Message),
		compressionJobs: make(chan compressionJob, 10),
		compressionResults: make(map[string]chan compressionResult),
	}
	
	// Start async compression worker
	go mc.compressionWorker()
	
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
		
		// Try async compression first with fallback to sync
		compressedMessages := mc.compressWithAIAsync(ctx, messages)
		if compressedMessages == nil {
			log.Printf("[WARN] Async compression failed, falling back to sync")
			compressedMessages = mc.compressWithAI(ctx, messages)
		}

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

	// Step 2: 使用AI压缩全部非系统消息
	compressedMessage := mc.createComprehensiveAISummary(ctx, nonSystemMessages)

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

// createComprehensiveAISummary creates a comprehensive AI summary preserving important context
func (mc *MessageCompressor) createComprehensiveAISummary(ctx context.Context, messages []llm.Message) *llm.Message {
	if mc.llmClient == nil {
		return nil
	}
	if len(messages) == 0 {
		return nil
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
		ModelType: mc.llmConfig.DefaultModelType,
		Config:    mc.llmConfig,
	}

	// Use shorter timeout to prevent blocking, with fallback handling
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	sessionID, _ := mc.sessionManager.GetSessionID()
	response, err := mc.llmClient.Chat(timeoutCtx, request, sessionID)
	if err != nil {
		log.Printf("[ERROR] MessageCompressor: Comprehensive AI summary failed: %v", err)
		return nil
	}

	if len(response.Choices) == 0 {
		log.Printf("[ERROR] MessageCompressor: No response choices from AI summary")
		return nil
	}
	// AI compression completed successfully
	return &llm.Message{
		Role:    "user",
		Content: fmt.Sprintf("Comprehensive conversation summary (%d messages): %s", len(messages), response.Choices[0].Message.Content),
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

// compressionWorker handles async compression jobs
func (mc *MessageCompressor) compressionWorker() {
	for job := range mc.compressionJobs {
		// Check cache first
		cacheKey := mc.getCacheKey(job.messages)
		mc.cacheMutex.RLock()
		cached, exists := mc.compressionCache[cacheKey]
		mc.cacheMutex.RUnlock()
		
		if exists && cached != nil {
			log.Printf("[INFO] Using cached compression result")
			job.resultCh <- compressionResult{
				messages: mc.rebuildWithCachedSummary(job.messages, cached),
				err:      nil,
			}
			continue
		}
		
		// Perform compression
		compressed := mc.compressWithAI(job.context, job.messages)
		if compressed != nil && len(compressed) > 0 {
			// Cache the summary message
			if len(compressed) > 2 {
				summaryMsg := &compressed[len(compressed)-1]
				mc.cacheMutex.Lock()
				mc.compressionCache[cacheKey] = summaryMsg
				mc.cacheMutex.Unlock()
			}
			
			job.resultCh <- compressionResult{
				messages: compressed,
				err:      nil,
			}
		} else {
			job.resultCh <- compressionResult{
				messages: nil,
				err:      fmt.Errorf("compression failed"),
			}
		}
	}
}

// compressWithAIAsync attempts async compression with timeout
func (mc *MessageCompressor) compressWithAIAsync(ctx context.Context, messages []llm.Message) []llm.Message {
	// Check cache first for quick return
	cacheKey := mc.getCacheKey(messages)
	mc.cacheMutex.RLock()
	cached, exists := mc.compressionCache[cacheKey]
	mc.cacheMutex.RUnlock()
	
	if exists && cached != nil {
		log.Printf("[INFO] Using cached compression result (fast path)")
		return mc.rebuildWithCachedSummary(messages, cached)
	}
	
	// Create job for async processing
	jobID := fmt.Sprintf("compress_%d", time.Now().UnixNano())
	resultCh := make(chan compressionResult, 1)
	
	mc.resultsMutex.Lock()
	mc.compressionResults[jobID] = resultCh
	mc.resultsMutex.Unlock()
	
	// Clean up after completion
	defer func() {
		mc.resultsMutex.Lock()
		delete(mc.compressionResults, jobID)
		mc.resultsMutex.Unlock()
		close(resultCh)
	}()
	
	// Submit job
	job := compressionJob{
		jobID:    jobID,
		context:  ctx,
		messages: messages,
		resultCh: resultCh,
	}
	
	select {
	case mc.compressionJobs <- job:
		// Job submitted successfully
	case <-time.After(1 * time.Second):
		// Queue is full, fallback to sync
		return nil
	}
	
	// Wait for result with timeout
	select {
	case result := <-resultCh:
		if result.err != nil {
			log.Printf("[WARN] Async compression failed: %v", result.err)
			return nil
		}
		return result.messages
	case <-time.After(25 * time.Second):
		// Timeout, return nil to trigger fallback
		log.Printf("[WARN] Async compression timed out")
		return nil
	case <-ctx.Done():
		return nil
	}
}

// getCacheKey generates a cache key for messages
func (mc *MessageCompressor) getCacheKey(messages []llm.Message) string {
	if len(messages) < 3 {
		return ""
	}
	
	// Use a hash of the message content for cache key
	var content strings.Builder
	for i := 2; i < len(messages) && i < len(messages)-2; i++ {
		content.WriteString(messages[i].Role)
		content.WriteString(":")
		if len(messages[i].Content) > 100 {
			content.WriteString(messages[i].Content[:100])
		} else {
			content.WriteString(messages[i].Content)
		}
		content.WriteString("|")
	}
	return fmt.Sprintf("%x", content.String())
}

// rebuildWithCachedSummary rebuilds message array with cached summary
func (mc *MessageCompressor) rebuildWithCachedSummary(messages []llm.Message, summary *llm.Message) []llm.Message {
	if len(messages) <= 2 {
		return messages
	}
	
	// Keep system messages + cached summary
	result := make([]llm.Message, 0, 3)
	result = append(result, messages[:2]...) // System messages
	result = append(result, *summary)        // Cached summary
	
	return result
}
