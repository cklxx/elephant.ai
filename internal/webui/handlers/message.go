package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"alex/internal/agent"
	"alex/internal/config"
	"alex/internal/session"
	"alex/pkg/types"
)

// MessageHandler - 消息处理器
type MessageHandler struct {
	reactAgent *agent.ReactAgent
	sessionMgr *session.Manager
}

// NewMessageHandler - 创建新的消息处理器
func NewMessageHandler(reactAgent *agent.ReactAgent, sessionMgr *session.Manager) *MessageHandler {
	return &MessageHandler{
		reactAgent: reactAgent,
		sessionMgr: sessionMgr,
	}
}

// SendMessage - 发送消息到会话
func (h *MessageHandler) SendMessage(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "session_id is required",
		})
		return
	}

	var req MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	if req.Content == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "message content is required",
		})
		return
	}

	// 确保会话存在
	_, err := h.reactAgent.RestoreSession(sessionID)
	if err != nil {
		// 如果会话不存在，尝试创建
		_, err = h.reactAgent.StartSession(sessionID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to create or restore session: %v", err),
			})
			return
		}
	}

	// 处理流式和非流式消息
	if req.StreamMode {
		// 对于流式模式，返回WebSocket连接信息
		streamURL := fmt.Sprintf("/api/sessions/%s/stream", sessionID)

		response := MessageResponse{
			SessionID: sessionID,
			StreamURL: streamURL,
		}

		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    response,
			Message: "Use WebSocket connection for streaming",
		})
		return
	}

	// 非流式处理
	cfg := &config.Config{}
	if req.Config != nil {
		// 简化的配置处理
		// 在实际应用中，你可能需要更复杂的配置转换逻辑
	}

	// 使用上下文
	ctx := context.WithValue(context.Background(), "session_id", sessionID)

	// 收集流式输出
	var streamChunks []agent.StreamChunk
	streamCallback := func(chunk agent.StreamChunk) {
		streamChunks = append(streamChunks, chunk)
	}

	// 处理消息
	err = h.reactAgent.ProcessMessageStream(ctx, req.Content, cfg, streamCallback)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to process message: %v", err),
		})
		return
	}

	// 获取更新后的会话
	_, err = h.reactAgent.RestoreSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to get updated session: %v", err),
		})
		return
	}

	// 构建响应
	response := MessageResponse{
		SessionID: sessionID,
		Response: &agent.Response{
			Message:     nil, // 最新消息会在session.Messages中
			ToolResults: []types.ReactToolResult{}, // 可以从streamChunks中提取
			SessionID:   sessionID,
			Complete:    true,
		},
	}

	// 添加会话信息到响应
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
		Message: "message processed successfully",
	})
}

// GetMessages - 获取会话消息历史
func (h *MessageHandler) GetMessages(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "session_id is required",
		})
		return
	}

	// 恢复会话
	sess, err := h.reactAgent.RestoreSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("session not found: %v", err),
		})
		return
	}

	// 获取分页参数
	limit := 50 // 默认限制
	if limitParam := c.Query("limit"); limitParam != "" {
		// 解析limit参数 (简化处理)
	}

	offset := 0 // 默认偏移
	if offsetParam := c.Query("offset"); offsetParam != "" {
		// 解析offset参数 (简化处理)
	}

	// 应用分页
	messages := sess.Messages
	total := len(messages)

	start := offset
	if start > total {
		start = total
	}

	end := start + limit
	if end > total {
		end = total
	}

	pagedMessages := messages[start:end]

	// 构建响应
	response := map[string]interface{}{
		"messages": pagedMessages,
		"total":    total,
		"offset":   offset,
		"limit":    limit,
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}