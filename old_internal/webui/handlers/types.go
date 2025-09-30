package handlers

import (
	"time"

	"alex/internal/agent"
	"alex/internal/session"
)

// APIResponse - 标准API响应格式
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SessionRequest - 创建会话请求
type SessionRequest struct {
	SessionID  string                 `json:"session_id,omitempty"`
	WorkingDir string                 `json:"working_dir,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
}

// SessionResponse - 会话响应
type SessionResponse struct {
	ID         string                 `json:"id"`
	Created    time.Time              `json:"created"`
	Updated    time.Time              `json:"updated"`
	WorkingDir string                 `json:"working_dir,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Messages   []*session.Message     `json:"messages,omitempty"`
}

// MessageRequest - 发送消息请求
type MessageRequest struct {
	Content    string                 `json:"content"`
	Config     map[string]interface{} `json:"config,omitempty"`
	StreamMode bool                   `json:"stream_mode,omitempty"`
}

// MessageResponse - 消息响应
type MessageResponse struct {
	SessionID string          `json:"session_id"`
	MessageID string          `json:"message_id,omitempty"`
	Response  *agent.Response `json:"response,omitempty"`
	StreamURL string          `json:"stream_url,omitempty"`
}

// ConfigUpdateRequest - 配置更新请求
type ConfigUpdateRequest struct {
	Config map[string]interface{} `json:"config"`
}
