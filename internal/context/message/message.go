package message

import (
	"context"
	"encoding/json"
	"math/rand"
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
}

// NewMessageProcessor 创建统一的消息处理器
func NewMessageProcessor(llmClient llm.Client, sessionManager *session.Manager, llmConfig *llm.Config) *MessageProcessor {

	return &MessageProcessor{
		sessionManager: sessionManager,
		tokenEstimator: NewTokenEstimator(),
		adapter:        message.NewAdapter(),                                       // 统一消息适配器
		compressor:     NewMessageCompressor(sessionManager, llmClient, llmConfig), // 简化的压缩器
	}
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
func (mp *MessageProcessor) ConvertSessionToLLM(sessionMessages []*session.Message) []llm.Message {
	llmMessages := make([]llm.Message, len(sessionMessages))
	for i, msg := range sessionMessages {
		llmMessages[i] = llm.Message{
			Role:       msg.Role,
			ToolCallId: msg.ToolCallId,
			Name:       msg.Name,
			Content:    msg.Content,
		}
		// 转换工具调用
		for _, tc := range msg.ToolCalls {
			// Convert map arguments to JSON string
			var argsStr string
			if tc.Function.Arguments != "" {
				if argsBytes, err := json.Marshal(tc.Function.Arguments); err == nil {
					argsStr = string(argsBytes)
				}
			}
			llmMessages[i].ToolCalls = append(llmMessages[i].ToolCalls, llm.ToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: llm.Function{
					Name:      tc.Function.Name,
					Arguments: argsStr,
				},
			})
		}
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
