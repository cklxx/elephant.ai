package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"alex/internal/config"
)

func TestNewServer(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器配置
	serverConfig := DefaultServerConfig()
	serverConfig.Port = 8081 // 使用不同的端口避免冲突

	// 创建服务器
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)
	assert.NotNil(t, server)
	assert.Equal(t, "localhost", server.host)
	assert.Equal(t, 8081, server.port)
}

func TestHealthEndpoint(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	// 创建测试请求
	req, err := http.NewRequest("GET", "/api/health", nil)
	require.NoError(t, err)

	// 创建响应记录器
	recorder := httptest.NewRecorder()

	// 执行请求
	server.engine.ServeHTTP(recorder, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, recorder.Code)

	var response APIResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// 验证健康检查数据
	healthData, ok := response.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "ok", healthData["status"])
	assert.Equal(t, "0.4.6", healthData["version"])
}

func TestCreateSession(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	// 创建会话请求
	sessionReq := SessionRequest{
		SessionID:  "test-session",
		WorkingDir: "/tmp",
	}

	reqBody, err := json.Marshal(sessionReq)
	require.NoError(t, err)

	// 创建测试请求
	req, err := http.NewRequest("POST", "/api/sessions", bytes.NewBuffer(reqBody))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	// 创建响应记录器
	recorder := httptest.NewRecorder()

	// 执行请求
	server.engine.ServeHTTP(recorder, req)

	// 验证响应
	assert.Equal(t, http.StatusCreated, recorder.Code)

	var response APIResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// 验证会话数据
	sessionData, ok := response.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test-session", sessionData["id"])
	assert.Equal(t, "/tmp", sessionData["working_dir"])
}

func TestListSessions(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	// 创建测试请求
	req, err := http.NewRequest("GET", "/api/sessions", nil)
	require.NoError(t, err)

	// 创建响应记录器
	recorder := httptest.NewRecorder()

	// 执行请求
	server.engine.ServeHTTP(recorder, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, recorder.Code)

	var response APIResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// 验证会话列表
	sessions, ok := response.Data.([]interface{})
	require.True(t, ok)
	assert.IsType(t, []interface{}{}, sessions)
}

func TestGetConfig(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	// 创建测试请求
	req, err := http.NewRequest("GET", "/api/config", nil)
	require.NoError(t, err)

	// 创建响应记录器
	recorder := httptest.NewRecorder()

	// 执行请求
	server.engine.ServeHTTP(recorder, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, recorder.Code)

	var response APIResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// 验证配置数据
	configData, ok := response.Data.(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, configData, "base_url")
	assert.Contains(t, configData, "model")
}

func TestGetTools(t *testing.T) {
	// 创建测试配置管理器
	configMgr, err := config.NewManager()
	require.NoError(t, err)

	// 创建服务器
	serverConfig := DefaultServerConfig()
	server, err := NewServer(configMgr, serverConfig)
	require.NoError(t, err)

	// 创建测试请求
	req, err := http.NewRequest("GET", "/api/tools", nil)
	require.NoError(t, err)

	// 创建响应记录器
	recorder := httptest.NewRecorder()

	// 执行请求
	server.engine.ServeHTTP(recorder, req)

	// 验证响应
	assert.Equal(t, http.StatusOK, recorder.Code)

	var response APIResponse
	err = json.Unmarshal(recorder.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, response.Success)

	// 验证工具数据
	toolsData, ok := response.Data.(map[string]interface{})
	require.True(t, ok)
	tools, ok := toolsData["tools"].([]interface{})
	require.True(t, ok)
	assert.IsType(t, []interface{}{}, tools)
}

func TestDefaultServerConfig(t *testing.T) {
	config := DefaultServerConfig()
	assert.Equal(t, "localhost", config.Host)
	assert.Equal(t, 8080, config.Port)
	assert.True(t, config.EnableCORS)
	assert.False(t, config.Debug)
	assert.Equal(t, 30*time.Second, config.ReadTimeout)
	assert.Equal(t, 30*time.Second, config.WriteTimeout)
}

func TestAPIResponseTypes(t *testing.T) {
	// 测试成功响应
	successResponse := APIResponse{
		Success: true,
		Data:    map[string]string{"key": "value"},
		Message: "success",
	}
	assert.True(t, successResponse.Success)
	assert.NotNil(t, successResponse.Data)
	assert.Equal(t, "success", successResponse.Message)

	// 测试错误响应
	errorResponse := APIResponse{
		Success: false,
		Error:   "test error",
	}
	assert.False(t, errorResponse.Success)
	assert.Equal(t, "test error", errorResponse.Error)
}