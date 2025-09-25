package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"alex/internal/agent"
	"alex/internal/config"
)

const (
	// WebSocket消息类型
	WSMsgTypeConnect     = "connect"
	WSMsgTypeDisconnect  = "disconnect"
	WSMsgTypeMessage     = "message"
	WSMsgTypeStream      = "stream"
	WSMsgTypeError       = "error"
	WSMsgTypeHeartbeat   = "heartbeat"
	WSMsgTypeComplete    = "complete"

	// WebSocket配置
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// WebSocketConnection 辅助方法

// safeCloseSend - 安全关闭Send通道
func (w *WebSocketConnection) safeCloseSend() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.sendClosed {
		close(w.Send)
		w.sendClosed = true
	}
}

// safeSendDone - 安全发送Done信号
func (w *WebSocketConnection) safeSendDone() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.doneClosed {
		select {
		case w.Done <- true:
		default:
			// Done channel已满或已关闭，跳过
		}
	}
}

// safeCloseDone - 安全关闭Done通道
func (w *WebSocketConnection) safeCloseDone() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.doneClosed {
		close(w.Done)
		w.doneClosed = true
	}
}

// handleWebSocket - WebSocket连接处理器
func (s *Server) handleWebSocket(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "session_id is required",
		})
		return
	}

	// 升级HTTP连接为WebSocket
	conn, err := s.wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "failed to upgrade to WebSocket",
		})
		return
	}

	// 创建WebSocket连接对象
	ctx, cancel := context.WithCancel(s.ctx)
	wsConn := &WebSocketConnection{
		Conn:      conn,
		SessionID: sessionID,
		Send:      make(chan WebSocketMessage, 256),
		Done:      make(chan bool),
		Context:   ctx,
		Cancel:    cancel,
	}

	// 添加到连接管理器
	s.addWebSocketConnection(sessionID, wsConn)

	// 启动WebSocket处理goroutines
	s.wg.Add(2)
	go s.wsReadPump(wsConn)
	go s.wsWritePump(wsConn)

	// 发送连接成功消息
	wsConn.Send <- WebSocketMessage{
		Type:      WSMsgTypeConnect,
		Data:      map[string]string{"session_id": sessionID},
		Timestamp: time.Now(),
		SessionID: sessionID,
	}

	log.Printf("WebSocket connection established for session: %s", sessionID)
}

// wsReadPump - WebSocket读取泵
func (s *Server) wsReadPump(wsConn *WebSocketConnection) {
	defer func() {
		s.wg.Done()
		s.removeWebSocketConnection(wsConn.SessionID)
		wsConn.Conn.Close()
		// 安全地发送Done信号
		wsConn.safeSendDone()
	}()

	// 设置读取配置
	wsConn.Conn.SetReadLimit(maxMessageSize)
	wsConn.Conn.SetReadDeadline(time.Now().Add(pongWait))
	wsConn.Conn.SetPongHandler(func(string) error {
		wsConn.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		select {
		case <-wsConn.Context.Done():
			return
		default:
			// 读取消息
			var msg WebSocketMessage
			err := wsConn.Conn.ReadJSON(&msg)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error: %v", err)
				}
				return
			}

			// 处理接收到的消息
			s.handleWebSocketMessage(wsConn, &msg)
		}
	}
}

// wsWritePump - WebSocket写入泵
func (s *Server) wsWritePump(wsConn *WebSocketConnection) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		s.wg.Done()
		ticker.Stop()
		wsConn.Conn.Close()
	}()

	for {
		select {
		case <-wsConn.Context.Done():
			return
		case message, ok := <-wsConn.Send:
			wsConn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				wsConn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := wsConn.Conn.WriteJSON(message); err != nil {
				log.Printf("WebSocket write error: %v", err)
				return
			}

		case <-ticker.C:
			wsConn.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := wsConn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleWebSocketMessage - 处理WebSocket消息
func (s *Server) handleWebSocketMessage(wsConn *WebSocketConnection, msg *WebSocketMessage) {
	switch msg.Type {
	case WSMsgTypeMessage:
		// 处理用户消息
		s.handleWebSocketMessageRequest(wsConn, msg)
	case WSMsgTypeHeartbeat:
		// 响应心跳
		s.sendWebSocketMessage(wsConn, WebSocketMessage{
			Type:      WSMsgTypeHeartbeat,
			Data:      map[string]string{"status": "alive"},
			Timestamp: time.Now(),
			SessionID: wsConn.SessionID,
		})
	default:
		log.Printf("Unknown WebSocket message type: %s", msg.Type)
	}
}

// handleWebSocketMessageRequest - 处理消息请求
func (s *Server) handleWebSocketMessageRequest(wsConn *WebSocketConnection, msg *WebSocketMessage) {
	// 解析消息数据
	var messageReq MessageRequest
	dataBytes, err := json.Marshal(msg.Data)
	if err != nil {
		s.sendWebSocketError(wsConn, "failed to parse message data")
		return
	}

	if err := json.Unmarshal(dataBytes, &messageReq); err != nil {
		s.sendWebSocketError(wsConn, "invalid message format")
		return
	}

	// 确保会话存在
	_, err = s.reactAgent.RestoreSession(wsConn.SessionID)
	if err != nil {
		// 如果会话不存在，创建新会话
		_, err = s.reactAgent.StartSession(wsConn.SessionID)
		if err != nil {
			s.sendWebSocketError(wsConn, "failed to create or restore session")
			return
		}
	}

	// 创建配置
	cfg := &config.Config{}
	if messageReq.Config != nil {
		// 将map转换为config (简化处理)
		cfgBytes, _ := json.Marshal(messageReq.Config)
		json.Unmarshal(cfgBytes, cfg)
	}

	// 创建流式回调函数
	streamCallback := func(chunk agent.StreamChunk) {
		// 转换StreamChunk为StreamMessage
		streamMsg := StreamMessage{
			Type:             chunk.Type,
			Content:          chunk.Content,
			Complete:         chunk.Complete,
			Metadata:         chunk.Metadata,
			TokensUsed:       chunk.TokensUsed,
			TotalTokensUsed:  chunk.TotalTokensUsed,
			PromptTokens:     chunk.PromptTokens,
			CompletionTokens: chunk.CompletionTokens,
			Timestamp:        time.Now(),
		}

		// 发送流式消息
		s.sendWebSocketMessage(wsConn, WebSocketMessage{
			Type:      WSMsgTypeStream,
			Data:      streamMsg,
			Timestamp: time.Now(),
			SessionID: wsConn.SessionID,
		})
	}

	// 异步处理消息
	go func() {
		ctx := context.WithValue(wsConn.Context, "session_id", wsConn.SessionID)
		err := s.reactAgent.ProcessMessageStream(ctx, messageReq.Content, cfg, streamCallback)
		if err != nil {
			s.sendWebSocketError(wsConn, fmt.Sprintf("failed to process message: %v", err))
			return
		}

		// 发送完成消息
		s.sendWebSocketMessage(wsConn, WebSocketMessage{
			Type:      WSMsgTypeComplete,
			Data:      map[string]string{"status": "completed"},
			Timestamp: time.Now(),
			SessionID: wsConn.SessionID,
		})
	}()
}

// sendWebSocketMessage - 发送WebSocket消息
func (s *Server) sendWebSocketMessage(wsConn *WebSocketConnection, msg WebSocketMessage) {
	select {
	case wsConn.Send <- msg:
	case <-wsConn.Context.Done():
		// 连接已关闭
	default:
		// 发送缓冲区已满，安全关闭连接
		wsConn.safeCloseSend()
	}
}

// sendWebSocketError - 发送WebSocket错误消息
func (s *Server) sendWebSocketError(wsConn *WebSocketConnection, errMsg string) {
	s.sendWebSocketMessage(wsConn, WebSocketMessage{
		Type:      WSMsgTypeError,
		Error:     errMsg,
		Timestamp: time.Now(),
		SessionID: wsConn.SessionID,
	})
}

// broadcastToSession - 向特定会话广播消息
func (s *Server) broadcastToSession(sessionID string, msg WebSocketMessage) {
	if conn, exists := s.getWebSocketConnection(sessionID); exists {
		s.sendWebSocketMessage(conn, msg)
	}
}

// CreateStreamCallback - 为指定会话创建流式回调函数
func (s *Server) CreateStreamCallback(sessionID string) agent.StreamCallback {
	return func(chunk agent.StreamChunk) {
		streamMsg := StreamMessage{
			Type:             chunk.Type,
			Content:          chunk.Content,
			Complete:         chunk.Complete,
			Metadata:         chunk.Metadata,
			TokensUsed:       chunk.TokensUsed,
			TotalTokensUsed:  chunk.TotalTokensUsed,
			PromptTokens:     chunk.PromptTokens,
			CompletionTokens: chunk.CompletionTokens,
			Timestamp:        time.Now(),
		}

		s.broadcastToSession(sessionID, WebSocketMessage{
			Type:      WSMsgTypeStream,
			Data:      streamMsg,
			Timestamp: time.Now(),
			SessionID: sessionID,
		})
	}
}