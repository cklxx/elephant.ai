package message

import (
	"context"
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
func NewMessageProcessor(llmClient llm.Client, sessionManager *session.Manager) *MessageProcessor {

	return &MessageProcessor{
		sessionManager: sessionManager,
		tokenEstimator: NewTokenEstimator(),
		adapter:        message.NewAdapter(),                            // 统一消息适配器
		compressor:     NewMessageCompressor(sessionManager, llmClient), // AI压缩器
	}
}

// ========== 消息压缩 ==========

// CompressMessages 使用AI压缩器压缩session消息
func (mp *MessageProcessor) CompressMessages(ctx context.Context, messages []*session.Message, actualTokens ...int) []*session.Message {
	return mp.compressor.CompressMessages(ctx, messages, actualTokens...)
}

// ========== 消息转换 ==========

// ConvertUnifiedToLLM 使用统一消息适配器将消息转换为LLM格式
func (mp *MessageProcessor) ConvertUnifiedToLLM(unifiedMessages []*message.Message) []llm.Message {
	unifiedLLMMessages := mp.adapter.ConvertToLLMMessages(unifiedMessages)
	llmMessages := make([]llm.Message, len(unifiedLLMMessages))
	for i, msg := range unifiedLLMMessages {
		llmMessages[i] = llm.Message{
			Role:             msg.Role,
			Content:          msg.Content,
			ToolCallId:       msg.ToolCallID,
			Name:             msg.Name,
			Reasoning:        msg.Reasoning,
			ReasoningSummary: msg.ReasoningSummary,
			Think:            msg.Think,
		}
		// 转换工具调用
		for _, tc := range msg.ToolCalls {
			llmMessages[i].ToolCalls = append(llmMessages[i].ToolCalls, llm.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: llm.Function{
					Name:        tc.Function.Name,
					Description: tc.Function.Description,
					Parameters:  tc.Function.Parameters,
					Arguments:   tc.Function.Arguments,
				},
			})
		}
	}
	return llmMessages
}

// ConvertLLMToUnified 使用统一消息适配器将LLM消息转换为统一格式
func (mp *MessageProcessor) ConvertLLMToUnified(llmMessages []llm.Message) []*message.Message {
	unifiedLLMMessages := make([]message.LLMMessage, len(llmMessages))
	for i, msg := range llmMessages {
		unifiedLLMMessages[i] = message.LLMMessage{
			Role:             msg.Role,
			Content:          msg.Content,
			ToolCallID:       msg.ToolCallId,
			Name:             msg.Name,
			Reasoning:        msg.Reasoning,
			ReasoningSummary: msg.ReasoningSummary,
			Think:            msg.Think,
		}
		// 转换工具调用
		for _, tc := range msg.ToolCalls {
			unifiedLLMMessages[i].ToolCalls = append(unifiedLLMMessages[i].ToolCalls, message.LLMToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: message.LLMFunction{
					Name:        tc.Function.Name,
					Description: tc.Function.Description,
					Parameters:  tc.Function.Parameters,
					Arguments:   tc.Function.Arguments,
				},
			})
		}
	}
	return mp.adapter.ConvertLLMMessages(unifiedLLMMessages)
}

// ConvertSessionToUnified 将session消息转换为统一消息格式
func (mp *MessageProcessor) ConvertSessionToUnified(sessionMessages []*session.Message) []*message.Message {
	sessionMsgs := make([]message.SessionMessage, len(sessionMessages))
	for i, msg := range sessionMessages {
		sessionMsgs[i] = message.SessionMessage{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolID:    msg.ToolID,
			Metadata:  msg.Metadata,
			Timestamp: msg.Timestamp,
		}
		// 转换工具调用
		for _, tc := range msg.ToolCalls {
			sessionMsgs[i].ToolCalls = append(sessionMsgs[i].ToolCalls, message.SessionToolCall{
				ID:   tc.ID,
				Name: tc.Name,
				Args: tc.Args,
			})
		}
	}
	return mp.adapter.ConvertSessionMessages(sessionMsgs)
}

// ConvertUnifiedToSession 将统一消息转换为session格式
func (mp *MessageProcessor) ConvertUnifiedToSession(unifiedMessages []*message.Message) []*session.Message {
	sessionMsgs := mp.adapter.ConvertToSessionMessages(unifiedMessages)
	messages := make([]*session.Message, len(sessionMsgs))
	for i, msg := range sessionMsgs {
		messages[i] = &session.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			ToolID:    msg.ToolID,
			Metadata:  msg.Metadata,
			Timestamp: msg.Timestamp,
		}
		// 转换工具调用
		for _, tc := range msg.ToolCalls {
			messages[i].ToolCalls = append(messages[i].ToolCalls, session.ToolCall{
				ID:   tc.ID,
				Name: tc.Name,
				Args: tc.Args,
			})
		}
	}
	return messages
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
