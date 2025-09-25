package webui

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"alex/internal/agent"
	"alex/internal/config"
	"alex/internal/session"
	"alex/internal/webui/handlers"
	"alex/internal/webui/middleware"
)

// Server - Web UI服务器
type Server struct {
	// 核心组件
	reactAgent  *agent.ReactAgent
	configMgr   *config.Manager
	sessionMgr  *session.Manager

	// HTTP服务器
	engine      *gin.Engine
	httpServer  *http.Server

	// WebSocket管理
	wsUpgrader  websocket.Upgrader
	wsConnections map[string]*WebSocketConnection
	wsConnMutex   sync.RWMutex

	// 服务器配置
	host        string
	port        int
	startTime   time.Time

	// 上下文控制
	ctx         context.Context
	cancel      context.CancelFunc

	// 并发控制
	wg          sync.WaitGroup
}

// ServerConfig - 服务器配置
type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	EnableCORS   bool   `json:"enable_cors"`
	Debug        bool   `json:"debug"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`
}

// DefaultServerConfig - 默认服务器配置
func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		Host:         "localhost",
		Port:         8080,
		EnableCORS:   true,
		Debug:        false,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

// NewServer - 创建新的Web UI服务器
func NewServer(configMgr *config.Manager, serverConfig *ServerConfig) (*Server, error) {
	// 创建ReactAgent实例
	reactAgent, err := agent.NewReactAgent(configMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to create ReactAgent: %w", err)
	}

	// 获取session manager
	sessionMgr := reactAgent.GetSessionManager()

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())

	// 设置Gin模式
	if !serverConfig.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	// 创建Gin引擎
	engine := gin.New()

	// 添加基础中间件
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	// 添加CORS支持
	if serverConfig.EnableCORS {
		corsConfig := cors.DefaultConfig()
		corsConfig.AllowAllOrigins = true
		corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
		corsConfig.AllowHeaders = []string{"Origin", "Content-Type", "Authorization", "X-Requested-With"}
		corsConfig.AllowWebSockets = true
		engine.Use(cors.New(corsConfig))
	}

	// WebSocket升级器配置
	wsUpgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// 在生产环境中应该添加更严格的源检查
			return true
		},
	}

	server := &Server{
		reactAgent:    reactAgent,
		configMgr:     configMgr,
		sessionMgr:    sessionMgr,
		engine:        engine,
		wsUpgrader:    wsUpgrader,
		wsConnections: make(map[string]*WebSocketConnection),
		host:          serverConfig.Host,
		port:          serverConfig.Port,
		startTime:     time.Now(),
		ctx:           ctx,
		cancel:        cancel,
	}

	// 创建HTTP服务器
	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", serverConfig.Host, serverConfig.Port),
		Handler:      engine,
		ReadTimeout:  serverConfig.ReadTimeout,
		WriteTimeout: serverConfig.WriteTimeout,
	}

	// 设置路由
	server.setupRoutes()

	return server, nil
}

// setupRoutes - 设置路由
func (s *Server) setupRoutes() {
	// 创建处理器实例
	sessionHandler := handlers.NewSessionHandler(s.reactAgent, s.sessionMgr)
	messageHandler := handlers.NewMessageHandler(s.reactAgent, s.sessionMgr)
	configHandler := handlers.NewConfigHandler(s.configMgr)

	// API路由组
	api := s.engine.Group("/api")
	api.Use(middleware.JSONMiddleware())
	api.Use(middleware.ErrorHandlingMiddleware())

	// 健康检查
	api.GET("/health", s.handleHealth)

	// 会话管理
	sessions := api.Group("/sessions")
	{
		sessions.POST("", sessionHandler.CreateSession)
		sessions.GET("", sessionHandler.ListSessions)
		sessions.GET("/:id", sessionHandler.GetSession)
		sessions.DELETE("/:id", sessionHandler.DeleteSession)
		sessions.POST("/:id/messages", messageHandler.SendMessage)
		sessions.GET("/:id/messages", messageHandler.GetMessages)
	}

	// WebSocket连接
	api.GET("/sessions/:id/stream", s.handleWebSocket)

	// 配置管理
	config := api.Group("/config")
	{
		config.GET("", configHandler.GetConfig)
		config.PUT("", configHandler.UpdateConfig)
	}

	// 工具管理
	tools := api.Group("/tools")
	{
		tools.GET("", s.handleGetTools)
	}

	// 静态文件服务 (如果需要的话)
	s.engine.Static("/static", "./web/static")
	s.engine.StaticFile("/", "./web/index.html")
}

// handleHealth - 健康检查处理器
func (s *Server) handleHealth(c *gin.Context) {
	uptime := time.Since(s.startTime)

	response := HealthResponse{
		Status:    "ok",
		Version:   "0.4.6", // 从ALEX版本获取
		Timestamp: time.Now(),
		Uptime:    uptime.String(),
	}

	c.JSON(http.StatusOK, handlers.APIResponse{
		Success: true,
		Data:    response,
	})
}

// handleGetTools - 获取工具列表处理器
func (s *Server) handleGetTools(c *gin.Context) {
	tools := s.reactAgent.GetAvailableTools(s.ctx)

	response := ToolsListResponse{
		Tools: tools,
	}

	c.JSON(http.StatusOK, handlers.APIResponse{
		Success: true,
		Data:    response,
	})
}

// Start - 启动服务器
func (s *Server) Start() error {
	log.Printf("Starting ALEX Web UI server on %s:%d", s.host, s.port)

	// 启动WebSocket连接管理器
	s.wg.Add(1)
	go s.manageWebSocketConnections()

	// 启动HTTP服务器
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start HTTP server: %w", err)
	}

	return nil
}

// Stop - 停止服务器
func (s *Server) Stop() error {
	log.Println("Stopping ALEX Web UI server...")

	// 取消上下文
	s.cancel()

	// 关闭所有WebSocket连接
	s.closeAllWebSocketConnections()

	// 停止HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down HTTP server: %v", err)
		return err
	}

	// 等待所有goroutine完成
	s.wg.Wait()

	log.Println("ALEX Web UI server stopped")
	return nil
}

// manageWebSocketConnections - 管理WebSocket连接
func (s *Server) manageWebSocketConnections() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			// 定期清理断开的连接
			s.cleanupWebSocketConnections()
		}
	}
}

// cleanupWebSocketConnections - 清理断开的WebSocket连接
func (s *Server) cleanupWebSocketConnections() {
	s.wsConnMutex.Lock()
	defer s.wsConnMutex.Unlock()

	for sessionID, conn := range s.wsConnections {
		select {
		case <-conn.Done:
			// 连接已关闭，从管理器中移除
			delete(s.wsConnections, sessionID)
			log.Printf("Cleaned up WebSocket connection for session: %s", sessionID)
		default:
			// 连接仍然活跃
		}
	}
}

// closeAllWebSocketConnections - 关闭所有WebSocket连接
func (s *Server) closeAllWebSocketConnections() {
	s.wsConnMutex.Lock()
	defer s.wsConnMutex.Unlock()

	for sessionID, conn := range s.wsConnections {
		conn.Cancel()
		conn.safeCloseDone()
		log.Printf("Closed WebSocket connection for session: %s", sessionID)
	}

	s.wsConnections = make(map[string]*WebSocketConnection)
}

// addWebSocketConnection - 添加WebSocket连接
func (s *Server) addWebSocketConnection(sessionID string, conn *WebSocketConnection) {
	s.wsConnMutex.Lock()
	defer s.wsConnMutex.Unlock()

	// 如果已存在连接，先关闭
	if existingConn, exists := s.wsConnections[sessionID]; exists {
		existingConn.Cancel()
		existingConn.safeCloseDone()
	}

	s.wsConnections[sessionID] = conn
	log.Printf("Added WebSocket connection for session: %s", sessionID)
}

// removeWebSocketConnection - 移除WebSocket连接
func (s *Server) removeWebSocketConnection(sessionID string) {
	s.wsConnMutex.Lock()
	defer s.wsConnMutex.Unlock()

	if conn, exists := s.wsConnections[sessionID]; exists {
		conn.Cancel()
		conn.safeCloseDone()
		delete(s.wsConnections, sessionID)
		log.Printf("Removed WebSocket connection for session: %s", sessionID)
	}
}

// GetWebSocketConnection - 获取WebSocket连接
func (s *Server) getWebSocketConnection(sessionID string) (*WebSocketConnection, bool) {
	s.wsConnMutex.RLock()
	defer s.wsConnMutex.RUnlock()

	conn, exists := s.wsConnections[sessionID]
	return conn, exists
}