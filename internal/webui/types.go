package webui

import (
	"context"
	"sync"
	"time"

	"alex/internal/agent"
	"alex/internal/config"
	"alex/internal/session"
	"github.com/gorilla/websocket"
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
	SessionID   string                 `json:"session_id,omitempty"`
	WorkingDir  string                 `json:"working_dir,omitempty"`
	Config      map[string]interface{} `json:"config,omitempty"`
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
	SessionID   string             `json:"session_id"`
	MessageID   string             `json:"message_id,omitempty"`
	Response    *agent.Response    `json:"response,omitempty"`
	StreamURL   string             `json:"stream_url,omitempty"`
}

// WebSocketMessage - WebSocket消息格式
type WebSocketMessage struct {
	Type      string             `json:"type"`
	Data      interface{}        `json:"data,omitempty"`
	Error     string             `json:"error,omitempty"`
	Timestamp time.Time          `json:"timestamp"`
	SessionID string             `json:"session_id,omitempty"`
}

// StreamMessage - 流式消息格式 (用于转换agent.StreamChunk)
type StreamMessage struct {
	Type             string         `json:"type"`
	Content          string         `json:"content"`
	Complete         bool           `json:"complete,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	TokensUsed       int            `json:"tokens_used,omitempty"`
	TotalTokensUsed  int            `json:"total_tokens_used,omitempty"`
	PromptTokens     int            `json:"prompt_tokens,omitempty"`
	CompletionTokens int            `json:"completion_tokens,omitempty"`
	Timestamp        time.Time      `json:"timestamp"`
}

// WebSocketConnection - WebSocket连接管理
type WebSocketConnection struct {
	Conn      *websocket.Conn
	SessionID string
	Send      chan WebSocketMessage
	Done      chan bool
	Context   context.Context
	Cancel    context.CancelFunc

	// 防止通道重复关闭
	sendClosed bool
	doneClosed bool
	mu         sync.Mutex
}

// SessionManager - Web UI专用的会话管理器
type WebUISessionManager struct {
	reactAgent    *agent.ReactAgent    // nolint:unused
	configMgr     *config.Manager     // nolint:unused
	sessions      map[string]*session.Session // nolint:unused
	wsConnections map[string]*WebSocketConnection // nolint:unused
	// Fields are used by the web UI for session and connection management
}

// ConfigUpdateRequest - 配置更新请求
type ConfigUpdateRequest struct {
	Config map[string]interface{} `json:"config"`
}

// ToolsListResponse - 工具列表响应
type ToolsListResponse struct {
	Tools []string `json:"tools"`
}

// HealthResponse - 健康检查响应
type HealthResponse struct {
	Status    string    `json:"status"`
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Uptime    string    `json:"uptime"`
}