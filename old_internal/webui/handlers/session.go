package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"alex/internal/agent"
	"alex/internal/session"
)

// SessionHandler - 会话处理器
type SessionHandler struct {
	reactAgent *agent.ReactAgent
	sessionMgr *session.Manager
}

// NewSessionHandler - 创建新的会话处理器
func NewSessionHandler(reactAgent *agent.ReactAgent, sessionMgr *session.Manager) *SessionHandler {
	return &SessionHandler{
		reactAgent: reactAgent,
		sessionMgr: sessionMgr,
	}
}

// CreateSession - 创建新会话
func (h *SessionHandler) CreateSession(c *gin.Context) {
	var req SessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid request: %v", err),
		})
		return
	}

	// 生成会话ID
	sessionID := req.SessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("session_%d", time.Now().UnixNano())
	}

	// 创建会话
	sess, err := h.reactAgent.StartSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to create session: %v", err),
		})
		return
	}

	// 如果提供了工作目录，设置会话的工作目录
	if req.WorkingDir != "" {
		sess.WorkingDir = req.WorkingDir
	}

	// 如果提供了配置，设置会话配置
	if req.Config != nil {
		sess.Config = req.Config
	}

	// 构建响应
	response := SessionResponse{
		ID:         sess.ID,
		Created:    sess.Created,
		Updated:    sess.Updated,
		WorkingDir: sess.WorkingDir,
		Config:     sess.Config,
		Messages:   sess.Messages,
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    response,
	})
}

// ListSessions - 获取会话列表
func (h *SessionHandler) ListSessions(c *gin.Context) {
	sessions, err := h.sessionMgr.ListSessionObjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to list sessions: %v", err),
		})
		return
	}

	// 转换为响应格式
	var responses []SessionResponse
	for _, sess := range sessions {
		responses = append(responses, SessionResponse{
			ID:         sess.ID,
			Created:    sess.Created,
			Updated:    sess.Updated,
			WorkingDir: sess.WorkingDir,
			Config:     sess.Config,
			Messages:   sess.Messages,
		})
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    responses,
	})
}

// GetSession - 获取单个会话
func (h *SessionHandler) GetSession(c *gin.Context) {
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

	// 构建响应
	response := SessionResponse{
		ID:         sess.ID,
		Created:    sess.Created,
		Updated:    sess.Updated,
		WorkingDir: sess.WorkingDir,
		Config:     sess.Config,
		Messages:   sess.Messages,
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}

// DeleteSession - 删除会话
func (h *SessionHandler) DeleteSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "session_id is required",
		})
		return
	}

	// 删除会话
	err := h.sessionMgr.DeleteSession(sessionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to delete session: %v", err),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "session deleted successfully",
	})
}
