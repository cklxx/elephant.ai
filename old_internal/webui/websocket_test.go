package webui

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"alex/internal/config"
)

func TestWebSocketConnection(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	// 首先创建一个会话
	sessionID := "test-websocket-session"
	_, err = server.reactAgent.StartSession(sessionID)
	require.NoError(t, err)

	// 创建测试服务器
	testServer := httptest.NewServer(server.engine)
	defer testServer.Close()

	// 将HTTP URL转换为WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/sessions/" + sessionID + "/stream"

	// 连接WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		// 安全关闭连接，忽略已关闭的连接错误
		if err := conn.Close(); err != nil && !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
			t.Logf("Failed to close WebSocket connection: %v", err)
		}
	}()

	// 设置读取超时
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("Failed to set read deadline: %v", err)
	}

	// 读取连接消息
	var msg WebSocketMessage
	err = conn.ReadJSON(&msg)
	require.NoError(t, err)
	assert.Equal(t, WSMsgTypeConnect, msg.Type)
	assert.Equal(t, sessionID, msg.SessionID)
}

func TestWebSocketHeartbeat(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	// 首先创建一个会话
	sessionID := "test-heartbeat-session"
	_, err = server.reactAgent.StartSession(sessionID)
	require.NoError(t, err)

	// 创建测试服务器
	testServer := httptest.NewServer(server.engine)
	defer testServer.Close()

	// 将HTTP URL转换为WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(testServer.URL, "http") + "/api/sessions/" + sessionID + "/stream"

	// 连接WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() {
		// 安全关闭连接，忽略已关闭的连接错误
		if err := conn.Close(); err != nil && !websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure, websocket.CloseNormalClosure) {
			t.Logf("Failed to close WebSocket connection: %v", err)
		}
	}()

	// 跳过连接消息
	var connectMsg WebSocketMessage
	err = conn.ReadJSON(&connectMsg)
	require.NoError(t, err)

	// 发送心跳消息
	heartbeatMsg := WebSocketMessage{
		Type:      WSMsgTypeHeartbeat,
		Timestamp: time.Now(),
		SessionID: sessionID,
	}

	err = conn.WriteJSON(heartbeatMsg)
	require.NoError(t, err)

	// 设置读取超时
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("Failed to set read deadline: %v", err)
	}

	// 读取心跳响应
	var responseMsg WebSocketMessage
	err = conn.ReadJSON(&responseMsg)
	require.NoError(t, err)
	assert.Equal(t, WSMsgTypeHeartbeat, responseMsg.Type)
	assert.Equal(t, sessionID, responseMsg.SessionID)
}

func TestWebSocketMessageTypes(t *testing.T) {
	// 测试WebSocket消息类型常量
	assert.Equal(t, "connect", WSMsgTypeConnect)
	assert.Equal(t, "disconnect", WSMsgTypeDisconnect)
	assert.Equal(t, "message", WSMsgTypeMessage)
	assert.Equal(t, "stream", WSMsgTypeStream)
	assert.Equal(t, "error", WSMsgTypeError)
	assert.Equal(t, "heartbeat", WSMsgTypeHeartbeat)
	assert.Equal(t, "complete", WSMsgTypeComplete)
}

func TestStreamMessage(t *testing.T) {
	// 测试流式消息结构
	streamMsg := StreamMessage{
		Type:             "thinking",
		Content:          "test content",
		Complete:         false,
		Metadata:         map[string]any{"phase": "test"},
		TokensUsed:       10,
		TotalTokensUsed:  100,
		PromptTokens:     50,
		CompletionTokens: 50,
		Timestamp:        time.Now(),
	}

	assert.Equal(t, "thinking", streamMsg.Type)
	assert.Equal(t, "test content", streamMsg.Content)
	assert.False(t, streamMsg.Complete)
	assert.Equal(t, 10, streamMsg.TokensUsed)
	assert.Equal(t, 100, streamMsg.TotalTokensUsed)
}

func TestWebSocketConnectionManagement(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	sessionID := "test-connection-mgmt"

	// 测试添加连接
	ctx, cancel := context.WithCancel(context.Background())
	wsConn := &WebSocketConnection{
		SessionID: sessionID,
		Send:      make(chan WebSocketMessage, 256),
		Done:      make(chan bool),
		Context:   ctx,
		Cancel:    cancel,
	}

	server.addWebSocketConnection(sessionID, wsConn)

	// 验证连接已添加
	retrievedConn, exists := server.getWebSocketConnection(sessionID)
	assert.True(t, exists)
	assert.Equal(t, sessionID, retrievedConn.SessionID)

	// 测试移除连接
	server.removeWebSocketConnection(sessionID)

	// 验证连接已移除
	_, exists = server.getWebSocketConnection(sessionID)
	assert.False(t, exists)
}
