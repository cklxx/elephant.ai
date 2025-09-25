import { apiClient } from './api-client'
import { webSocketClient } from './websocket-client'
import { useAppStore } from '@/stores'

export interface ConnectionConfig {
  apiUrl?: string
  wsUrl?: string
  enableWebSocket?: boolean
  autoReconnect?: boolean
  healthCheckInterval?: number
}

class ConnectionManager {
  private config: Required<ConnectionConfig>
  private healthCheckTimer?: NodeJS.Timeout
  private isInitialized = false

  constructor() {
    this.config = {
      apiUrl: 'http://localhost:8080/api',
      wsUrl: 'ws://localhost:8080/api/sessions',
      enableWebSocket: true,
      autoReconnect: true,
      healthCheckInterval: 30000, // 30 seconds
    }
  }

  async initialize(config?: Partial<ConnectionConfig>): Promise<void> {
    if (this.isInitialized) {
      return
    }

    this.config = { ...this.config, ...config }

    try {
      // Test API connection
      await this.testApiConnection()

      // WebSocket connections are now session-specific

      // Start health checks
      this.startHealthChecks()

      this.isInitialized = true
      useAppStore.getState().setConnected(true)
    } catch (error) {
      console.error('Connection initialization failed:', error)
      useAppStore.getState().setConnectionError(
        error instanceof Error ? error.message : 'Connection failed'
      )
      throw error
    }
  }

  async shutdown(): Promise<void> {
    this.stopHealthChecks()

    if (webSocketClient.isConnected()) {
      webSocketClient.disconnect()
    }

    this.isInitialized = false
    useAppStore.getState().setConnected(false)
  }

  private async testApiConnection(): Promise<void> {
    try {
      const health = await apiClient.health()
      console.log('API connection successful:', health)
    } catch (error) {
      throw new Error(`API connection failed: ${error}`)
    }
  }


  private startHealthChecks(): void {
    this.healthCheckTimer = setInterval(async () => {
      try {
        await apiClient.health()
        useAppStore.getState().setConnected(true)
      } catch (error) {
        console.error('Health check failed:', error)
        useAppStore.getState().setConnectionError('Connection lost')
      }
    }, this.config.healthCheckInterval)
  }

  private stopHealthChecks(): void {
    if (this.healthCheckTimer) {
      clearInterval(this.healthCheckTimer)
      this.healthCheckTimer = undefined
    }
  }

  isConnected(): boolean {
    return this.isInitialized && useAppStore.getState().isConnected
  }

  getConfig(): ConnectionConfig {
    return { ...this.config }
  }

  // 为指定会话建立WebSocket连接
  async connectWebSocketForSession(sessionId: string): Promise<void> {
    if (!this.config.enableWebSocket) {
      return
    }

    try {
      // 先断开任何现有连接
      if (webSocketClient.isConnected()) {
        webSocketClient.disconnect()
      }

      // 构建会话特定的WebSocket URL
      const sessionWsUrl = `${this.config.wsUrl}/${sessionId}/stream`

      // 设置事件处理器
      webSocketClient.onConnect(() => {
        console.log(`WebSocket connected for session: ${sessionId}`)
        useAppStore.getState().setConnected(true)
      })

      webSocketClient.onDisconnect(reason => {
        console.log('WebSocket disconnected:', reason)
        if (this.config.autoReconnect && reason !== 'io client disconnect') {
          // 重连逻辑由客户端处理
        }
      })

      webSocketClient.onError(error => {
        console.error('WebSocket error:', error)
        useAppStore.getState().setConnectionError(error)
      })

      webSocketClient.onMessage(chunk => {
        console.log('Streaming chunk received:', chunk)
        // 消息由message store处理
      })

      // 连接到会话特定的WebSocket端点
      await webSocketClient.connect(sessionWsUrl)
    } catch (error) {
      console.warn('Session WebSocket connection failed, falling back to HTTP:', error)
      // 继续使用HTTP API
    }
  }

  // 断开WebSocket连接
  disconnectWebSocket(): void {
    if (webSocketClient.isConnected()) {
      webSocketClient.disconnect()
    }
  }
}

export const connectionManager = new ConnectionManager()