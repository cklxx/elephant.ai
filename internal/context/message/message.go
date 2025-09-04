package message

import (
	"context"
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"alex/internal/llm"
	"alex/internal/session"
	"alex/pkg/types/message"
)

// MessageProcessor 统一的消息处理器，整合所有消息相关功能
type MessageProcessor struct {
	sessionManager *session.Manager
	tokenEstimator *TokenEstimator
	adapter        *message.Adapter   // 统一消息适配器
	compressor     *MessageCompressor // AI压缩器
	
	// Object pools for reducing allocations
	messagePool    sync.Pool
	toolCallPool   sync.Pool
	slicePool      sync.Pool
	jsonBufPool    sync.Pool
}

// NewMessageProcessor 创建统一的消息处理器
func NewMessageProcessor(llmClient llm.Client, sessionManager *session.Manager, llmConfig *llm.Config) *MessageProcessor {
	mp := &MessageProcessor{
		sessionManager: sessionManager,
		tokenEstimator: NewTokenEstimator(),
		adapter:        message.NewAdapter(),
		compressor:     NewMessageCompressor(sessionManager, llmClient, llmConfig),
	}
	
	// Initialize object pools
	mp.messagePool = sync.Pool{
		New: func() interface{} {
			return &llm.Message{}
		},
	}
	
	mp.toolCallPool = sync.Pool{
		New: func() interface{} {
			return &llm.ToolCall{}
		},
	}
	
	mp.slicePool = sync.Pool{
		New: func() interface{} {
			return make([]llm.Message, 0, 10)
		},
	}
	
	mp.jsonBufPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 0, 1024)
		},
	}
	
	return mp
}

// ========== 消息压缩 ==========

// CompressMessages 使用AI压缩器压缩LLM消息
// consumedTokens: 累积消耗的token数
// currentTokens: 当前消息的token数（压缩后会重置为0）
func (mp *MessageProcessor) CompressMessages(ctx context.Context, messages []llm.Message, consumedTokens int, currentTokens int) ([]llm.Message, int, int) {
	return mp.compressor.CompressMessages(ctx, messages, consumedTokens, currentTokens)
}

// ========== 消息转换 ==========

// ConvertSessionToLLM 将Session消息转换为LLM格式（仅用于session历史加载）
// 使用对象池优化减少内存分配
func (mp *MessageProcessor) ConvertSessionToLLM(sessionMessages []*session.Message) []llm.Message {
	if len(sessionMessages) == 0 {
		return nil
	}
	
	return mp.convertSessionToLLMOptimized(sessionMessages)
}

// convertSessionToLLMOptimized provides optimized implementation with object pooling
func (mp *MessageProcessor) convertSessionToLLMOptimized(sessionMessages []*session.Message) []llm.Message {
	// Use pooled slice if possible
	var llmMessages []llm.Message
	if pooledSlice := mp.slicePool.Get(); pooledSlice != nil {
		if slice, ok := pooledSlice.([]llm.Message); ok {
			// Reset slice but keep capacity
			slice = slice[:0]
			if cap(slice) >= len(sessionMessages) {
				llmMessages = slice
			}
		}
	}
	
	// Fallback to regular allocation if pool doesn't fit
	if llmMessages == nil {
		llmMessages = make([]llm.Message, 0, len(sessionMessages))
	}
	
	// Convert messages using pools to reduce allocations
	for _, msg := range sessionMessages {
		// Get message from pool
		pooledMsg := mp.messagePool.Get().(*llm.Message)
		
		// Reset and populate message
		*pooledMsg = llm.Message{}
		pooledMsg.Role = msg.Role
		pooledMsg.ToolCallId = msg.ToolCallId
		pooledMsg.Name = msg.Name
		pooledMsg.Content = msg.Content
		
		// Convert tool calls with pooling
		pooledMsg.ToolCalls = mp.convertToolCallsOptimized(msg.ToolCalls)
		
		// Copy to result slice and return to pool
		llmMessages = append(llmMessages, *pooledMsg)
		mp.messagePool.Put(pooledMsg)
	}
	
	return llmMessages
}

// ========== 随机消息生成 ==========

var processingMessages = []string{
	"Processing", "Thinking", "Learning", "Exploring", "Discovering",
	"Analyzing", "Computing", "Reasoning", "Planning", "Executing",
	"Optimizing", "Searching", "Understanding", "Crafting", "Creating",
	"Parsing", "Generating", "Evaluating", "Calculating", "Investigating",
}

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// GetRandomProcessingMessage 获取随机处理消息
func GetRandomProcessingMessage() string {
	return "👾 " + processingMessages[rng.Intn(len(processingMessages))] + "..."
}

// GetRandomProcessingMessageWithEmoji 获取带emoji的随机处理消息
func GetRandomProcessingMessageWithEmoji() string {
	return "⚡ " + GetRandomProcessingMessage() + " please wait"
}

// convertToolCallsOptimized converts tool calls with reduced allocations
func (mp *MessageProcessor) convertToolCallsOptimized(toolCalls []llm.ToolCall) []llm.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	
	result := make([]llm.ToolCall, 0, len(toolCalls))
	
	for _, tc := range toolCalls {
		// Get pooled tool call
		pooledTC := mp.toolCallPool.Get().(*llm.ToolCall)
		*pooledTC = llm.ToolCall{}
		
		pooledTC.ID = tc.ID
		pooledTC.Type = "function"
		pooledTC.Function.Name = tc.Function.Name
		
		// Optimize JSON marshaling using pool
		if tc.Function.Arguments != "" {
			if argsStr := mp.marshalArgumentsOptimized(tc.Function.Arguments); argsStr != "" {
				pooledTC.Function.Arguments = argsStr
			}
		}
		
		// Copy to result and return to pool
		result = append(result, *pooledTC)
		mp.toolCallPool.Put(pooledTC)
	}
	
	return result
}

// marshalArgumentsOptimized marshals arguments with pooled buffer
func (mp *MessageProcessor) marshalArgumentsOptimized(args interface{}) string {
	if args == nil {
		return ""
	}
	
	// Get pooled buffer
	buf := mp.jsonBufPool.Get().([]byte)
	buf = buf[:0] // Reset length but keep capacity
	
	defer mp.jsonBufPool.Put(buf)
	
	// Try to marshal directly to the buffer
	if data, err := json.Marshal(args); err == nil {
		return string(data)
	}
	
	return ""
}

// ReleaseConvertedMessages returns pooled resources (call after using converted messages)
func (mp *MessageProcessor) ReleaseConvertedMessages(messages []llm.Message) {
	if len(messages) == 0 {
		return
	}
	
	// Return slice to pool if it's a reasonable size
	if cap(messages) <= 100 {
		messages = messages[:0] // Reset length but keep capacity
		mp.slicePool.Put(messages)
	}
}
