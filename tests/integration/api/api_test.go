package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"alex/internal/config"
	"alex/internal/webui"
)

// APITestSuite API集成测试套件
type APITestSuite struct {
	suite.Suite

	// Test dependencies
	configMgr   *config.Manager
	server      *webui.Server

	// Test server
	testServer  *httptest.Server
	client      *http.Client
	baseURL     string
}

// API请求/响应结构体
type APIRequest struct {
	Message   string                 `json:"message"`
	SessionID string                 `json:"session_id,omitempty"`
	Stream    bool                   `json:"stream,omitempty"`
	Options   map[string]interface{} `json:"options,omitempty"`
}

type APIResponse struct {
	Success   bool                   `json:"success"`
	Data      interface{}            `json:"data,omitempty"`
	Error     string                 `json:"error,omitempty"`
	SessionID string                 `json:"session_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type StreamMessage struct {
	Type      string      `json:"type"`
	Content   string      `json:"content,omitempty"`
	SessionID string      `json:"session_id,omitempty"`
	Data      interface{} `json:"data,omitempty"`
	Done      bool        `json:"done,omitempty"`
}

// SetupSuite 测试套件初始化
func (suite *APITestSuite) SetupSuite() {
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

	// 创建WebUI服务器 - 使用不同端口避免冲突
	serverConfig := webui.DefaultServerConfig()
	serverConfig.Port = 18081  // 使用测试端口
	serverConfig.Debug = true // 启用调试模式进行测试

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
	suite.baseURL = suite.testServer.URL
	suite.client = &http.Client{Timeout: time.Second * 10}
}

// TearDownSuite 清理测试环境
func (suite *APITestSuite) TearDownSuite() {
	if suite.server != nil {
		if err := suite.server.Stop(); err != nil {
			suite.T().Logf("Server stop error: %v", err)
		}
	}
}

// SetupTest 每个测试前的设置
func (suite *APITestSuite) SetupTest() {
	// 清理状态，如果有需要的话
}

// TestHealthCheck 测试健康检查端点
func (suite *APITestSuite) TestHealthCheck() {
	resp, err := suite.client.Get(suite.baseURL + "/api/health")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
}

// TestCreateSession 测试创建会话
func (suite *APITestSuite) TestCreateSession() {
	requestData := map[string]interface{}{
		"session_id": "test-session-1",
	}

	jsonData, err := json.Marshal(requestData)
	require.NoError(suite.T(), err)

	resp, err := suite.client.Post(suite.baseURL+"/api/sessions", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
	assert.Equal(suite.T(), "test-session-1", response.SessionID)
}

// TestListSessions 测试获取会话列表
func (suite *APITestSuite) TestListSessions() {
	// 先创建一个会话
	suite.TestCreateSession()

	resp, err := suite.client.Get(suite.baseURL + "/api/sessions")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)

	// 验证返回的数据格式
	if sessions, ok := response.Data.([]interface{}); ok {
		assert.GreaterOrEqual(suite.T(), len(sessions), 1)
	}
}

// TestSendMessage 测试发送消息
func (suite *APITestSuite) TestSendMessage() {
	// 先创建会话
	sessionID := "test-session-send"
	suite.createTestSession(sessionID)

	// 发送消息
	requestData := APIRequest{
		Message:   "Hello, ALEX!",
		SessionID: sessionID,
		Stream:    false,
	}

	jsonData, err := json.Marshal(requestData)
	require.NoError(suite.T(), err)

	resp, err := suite.client.Post(
		suite.baseURL+"/api/sessions/"+sessionID+"/messages",
		"application/json",
		bytes.NewBuffer(jsonData),
	)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
}

// TestGetMessages 测试获取消息历史
func (suite *APITestSuite) TestGetMessages() {
	sessionID := "test-session-messages"
	suite.createTestSession(sessionID)

	resp, err := suite.client.Get(suite.baseURL + "/api/sessions/" + sessionID + "/messages")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
}

// TestDeleteSession 测试删除会话
func (suite *APITestSuite) TestDeleteSession() {
	sessionID := "test-session-delete"
	suite.createTestSession(sessionID)

	req, err := http.NewRequest("DELETE", suite.baseURL+"/api/sessions/"+sessionID, nil)
	require.NoError(suite.T(), err)

	resp, err := suite.client.Do(req)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
}

// TestGetConfig 测试获取配置
func (suite *APITestSuite) TestGetConfig() {
	resp, err := suite.client.Get(suite.baseURL + "/api/config")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
}

// TestGetTools 测试获取工具列表
func (suite *APITestSuite) TestGetTools() {
	resp, err := suite.client.Get(suite.baseURL + "/api/tools")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)

	var response APIResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), response.Success)
}

// TestConcurrentSessions 测试并发会话处理
func (suite *APITestSuite) TestConcurrentSessions() {
	const numSessions = 5
	sessionIDs := make([]string, numSessions)

	// 并发创建多个会话
	for i := 0; i < numSessions; i++ {
		sessionIDs[i] = fmt.Sprintf("concurrent-session-%d", i)
		go suite.createTestSession(sessionIDs[i])
	}

	// 等待所有会话创建完成
	time.Sleep(time.Second * 2)

	// 验证所有会话都已创建
	for _, sessionID := range sessionIDs {
		resp, err := suite.client.Get(suite.baseURL + "/api/sessions/" + sessionID)
		require.NoError(suite.T(), err)
		resp.Body.Close()
		assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	}
}

// TestErrorHandling 测试错误处理
func (suite *APITestSuite) TestErrorHandling() {
	// 测试不存在的会话
	resp, err := suite.client.Get(suite.baseURL + "/api/sessions/non-existent-session")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusNotFound, resp.StatusCode)

	// 测试无效的消息格式
	invalidData := []byte(`{"invalid": json}`)
	resp, err = suite.client.Post(
		suite.baseURL+"/api/sessions/test/messages",
		"application/json",
		bytes.NewBuffer(invalidData),
	)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

// Helper methods

// createTestSession 创建测试会话
func (suite *APITestSuite) createTestSession(sessionID string) {
	requestData := map[string]interface{}{
		"session_id": sessionID,
	}

	jsonData, err := json.Marshal(requestData)
	require.NoError(suite.T(), err)

	resp, err := suite.client.Post(suite.baseURL+"/api/sessions", "application/json", bytes.NewBuffer(jsonData))
	require.NoError(suite.T(), err)
	defer resp.Body.Close()

	require.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

// 运行测试套件
func TestAPIIntegration(t *testing.T) {
	suite.Run(t, new(APITestSuite))
}