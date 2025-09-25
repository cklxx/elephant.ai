package websocket

import (
	"fmt"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"alex/internal/agent"
	"alex/internal/config"
	"alex/internal/session"
	"alex/internal/webui"
)

// WebSocketTestSuite WebSocket集成测试套件
type WebSocketTestSuite struct {
	suite.Suite

	// Test dependencies
	configMgr   *config.Manager
	sessionMgr  *session.Manager
	agent       *agent.ReactAgent
	server      *webui.Server

	// Test server
	testServer  *httptest.Server
	client      *websocket.Conn
}

// WebSocket消息结构
type WSMessage struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id"`
	Content   string                 `json:"content,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

type WSResponse struct {
	Type      string                 `json:"type"`
	SessionID string                 `json:"session_id"`
	Content   string                 `json:"content,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Done      bool                   `json:"done,omitempty"`
}

// SetupSuite 测试套件初始化
func (suite *WebSocketTestSuite) SetupSuite() {
	// 创建配置管理器
	configMgr, err := config.NewManager()
	require.NoError(suite.T(), err)

	// 设置测试配置
	err = configMgr.Set("api_key", "test-api-key")
	require.NoError(suite.T(), err)
	err = configMgr.Set("base_url", "https://api.test.com")
	require.NoError(suite.T(), err)
	err = configMgr.Set("model", "test-model")
	require.NoError(suite.T(), err)
	err = configMgr.Set("max_tokens", 1000)
	require.NoError(suite.T(), err)
	err = configMgr.Set("temperature", 0.7)
	require.NoError(suite.T(), err)

	suite.configMgr = configMgr

	// 创建会话管理器
	sessionMgr, err := session.NewManager()
	require.NoError(suite.T(), err)
	suite.sessionMgr = sessionMgr

	// 创建ReactAgent
	agent, err := agent.NewReactAgent(configMgr)
	require.NoError(suite.T(), err)
	suite.agent = agent

	// 创建WebUI服务器 - 使用不同端口避免冲突
	serverConfig := webui.DefaultServerConfig()
	serverConfig.Port = 18080  // 使用测试端口
	server, err := webui.NewServer(configMgr, serverConfig)
	require.NoError(suite.T(), err)
	suite.server = server

	// 在后台启动服务器
	go func() {
		if err := server.Start(); err != nil {
			suite.T().Logf("Server start error: %v", err)
		}
	}()

	// 等待服务器启动
	time.Sleep(500 * time.Millisecond)

	// 设置测试服务器URL
	suite.testServer = &httptest.Server{
		URL: fmt.Sprintf("http://%s:%d", serverConfig.Host, serverConfig.Port),
	}
}

// TearDownSuite 清理测试环境
func (suite *WebSocketTestSuite) TearDownSuite() {
	if suite.client != nil {
		suite.client.Close()
	}
	if suite.server != nil {
		if err := suite.server.Stop(); err != nil {
			suite.T().Logf("Server stop error: %v", err)
		}
	}
}

// SetupTest 每个测试前的设置
func (suite *WebSocketTestSuite) SetupTest() {
	// 清理状态
}

// TearDownTest 每个测试后的清理
func (suite *WebSocketTestSuite) TearDownTest() {
	if suite.client != nil {
		suite.client.Close()
		suite.client = nil
	}
}

// TestWebSocketConnection 测试WebSocket连接
func (suite *WebSocketTestSuite) TestWebSocketConnection() {
	sessionID := "test-ws-connection"

	// 建立WebSocket连接
	conn, err := suite.connectWebSocket(sessionID)
	require.NoError(suite.T(), err)
	defer conn.Close()

	// 发送连接消息
	message := WSMessage{
		Type:      "connect",
		SessionID: sessionID,
	}

	err = conn.WriteJSON(message)
	require.NoError(suite.T(), err)

	// 读取响应
	var response WSResponse
	err = conn.ReadJSON(&response)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "connected", response.Type)
	assert.Equal(suite.T(), sessionID, response.SessionID)
}

// TestWebSocketStreaming 测试WebSocket流式响应
func (suite *WebSocketTestSuite) TestWebSocketStreaming() {
	sessionID := "test-ws-streaming"

	conn, err := suite.connectWebSocket(sessionID)
	require.NoError(suite.T(), err)
	defer conn.Close()

	// 发送消息
	message := WSMessage{
		Type:      "message",
		SessionID: sessionID,
		Content:   "Hello, ALEX!",
	}

	err = conn.WriteJSON(message)
	require.NoError(suite.T(), err)

	// 读取流式响应
	responses := make([]WSResponse, 0)
	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-timeout:
			suite.T().Fatal("Timeout waiting for WebSocket response")
		default:
			var response WSResponse
			err = conn.ReadJSON(&response)
			if err != nil {
				suite.T().Logf("WebSocket read error: %v", err)
				continue
			}

			responses = append(responses, response)

			if response.Done {
				goto done
			}
		}
	}

done:
	assert.Greater(suite.T(), len(responses), 0)
	assert.True(suite.T(), responses[len(responses)-1].Done)
}

// TestWebSocketMultipleClients 测试多个WebSocket客户端
func (suite *WebSocketTestSuite) TestWebSocketMultipleClients() {
	const numClients = 3
	connections := make([]*websocket.Conn, numClients)
	sessionIDs := make([]string, numClients)

	// 创建多个连接
	for i := 0; i < numClients; i++ {
		sessionIDs[i] = fmt.Sprintf("test-ws-multi-%d", i)
		conn, err := suite.connectWebSocket(sessionIDs[i])
		require.NoError(suite.T(), err)
		connections[i] = conn
		defer conn.Close()
	}

	// 每个客户端发送消息
	for i, conn := range connections {
		message := WSMessage{
			Type:      "message",
			SessionID: sessionIDs[i],
			Content:   fmt.Sprintf("Message from client %d", i),
		}

		err := conn.WriteJSON(message)
		require.NoError(suite.T(), err)
	}

	// 验证每个客户端都收到响应
	for i, conn := range connections {
		var response WSResponse
		err := conn.ReadJSON(&response)
		require.NoError(suite.T(), err)
		assert.Equal(suite.T(), sessionIDs[i], response.SessionID)
	}
}

// TestWebSocketHeartbeat 测试WebSocket心跳
func (suite *WebSocketTestSuite) TestWebSocketHeartbeat() {
	sessionID := "test-ws-heartbeat"

	conn, err := suite.connectWebSocket(sessionID)
	require.NoError(suite.T(), err)
	defer conn.Close()

	// 发送心跳消息
	message := WSMessage{
		Type:      "ping",
		SessionID: sessionID,
	}

	err = conn.WriteJSON(message)
	require.NoError(suite.T(), err)

	// 等待pong响应
	var response WSResponse
	err = conn.ReadJSON(&response)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "pong", response.Type)
	assert.Equal(suite.T(), sessionID, response.SessionID)
}

// TestWebSocketError 测试WebSocket错误处理
func (suite *WebSocketTestSuite) TestWebSocketError() {
	sessionID := "test-ws-error"

	conn, err := suite.connectWebSocket(sessionID)
	require.NoError(suite.T(), err)
	defer conn.Close()

	// 发送无效消息
	message := WSMessage{
		Type:      "invalid_type",
		SessionID: sessionID,
	}

	err = conn.WriteJSON(message)
	require.NoError(suite.T(), err)

	// 读取错误响应
	var response WSResponse
	err = conn.ReadJSON(&response)
	require.NoError(suite.T(), err)

	assert.Equal(suite.T(), "error", response.Type)
	assert.NotEmpty(suite.T(), response.Error)
}

// TestWebSocketReconnection 测试WebSocket重连
func (suite *WebSocketTestSuite) TestWebSocketReconnection() {
	sessionID := "test-ws-reconnect"

	// 第一次连接
	conn1, err := suite.connectWebSocket(sessionID)
	require.NoError(suite.T(), err)

	// 发送消息
	message := WSMessage{
		Type:      "message",
		SessionID: sessionID,
		Content:   "First connection",
	}

	err = conn1.WriteJSON(message)
	require.NoError(suite.T(), err)

	// 关闭连接
	conn1.Close()

	// 重新连接
	conn2, err := suite.connectWebSocket(sessionID)
	require.NoError(suite.T(), err)
	defer conn2.Close()

	// 发送另一条消息
	message = WSMessage{
		Type:      "message",
		SessionID: sessionID,
		Content:   "Second connection",
	}

	err = conn2.WriteJSON(message)
	require.NoError(suite.T(), err)

	// 验证消息被处理
	var response WSResponse
	err = conn2.ReadJSON(&response)
	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), sessionID, response.SessionID)
}

// TestWebSocketToolExecution 测试通过WebSocket执行工具
func (suite *WebSocketTestSuite) TestWebSocketToolExecution() {
	sessionID := "test-ws-tools"

	conn, err := suite.connectWebSocket(sessionID)
	require.NoError(suite.T(), err)
	defer conn.Close()

	// 发送工具执行请求
	message := WSMessage{
		Type:      "message",
		SessionID: sessionID,
		Content:   "Execute file_read tool to read this file",
	}

	err = conn.WriteJSON(message)
	require.NoError(suite.T(), err)

	// 收集所有响应直到完成
	var toolExecutions []WSResponse
	timeout := time.After(15 * time.Second)

	for {
		select {
		case <-timeout:
			suite.T().Fatal("Timeout waiting for tool execution")
		default:
			var response WSResponse
			err = conn.ReadJSON(&response)
			if err != nil {
				continue
			}

			if response.Type == "tool_start" || response.Type == "tool_result" {
				toolExecutions = append(toolExecutions, response)
			}

			if response.Done {
				goto toolDone
			}
		}
	}

toolDone:
	// 验证工具执行
	assert.Greater(suite.T(), len(toolExecutions), 0)
}

// Helper methods

// connectWebSocket 建立WebSocket连接
func (suite *WebSocketTestSuite) connectWebSocket(sessionID string) (*websocket.Conn, error) {
	// 构建WebSocket URL
	wsURL := url.URL{
		Scheme: "ws",
		Host:   suite.testServer.URL[7:], // 去掉 "http://"
		Path:   "/api/sessions/" + sessionID + "/stream",
	}

	// 建立WebSocket连接
	conn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), nil)
	if err != nil {
		return nil, err
	}

	suite.client = conn
	return conn, nil
}

// 运行测试套件
func TestWebSocketIntegration(t *testing.T) {
	suite.Run(t, new(WebSocketTestSuite))
}