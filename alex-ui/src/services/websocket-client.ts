import { io, Socket } from 'socket.io-client'
import {
  WebSocketClient,
  WebSocketEvent,
  StreamingChunk,
  StreamingChunkSchema,
} from '@/types'

class WebSocketClientImpl implements WebSocketClient {
  private socket?: Socket
  private url?: string
  private reconnectAttempts = 0
  private maxReconnectAttempts = 5
  private reconnectDelay = 1000

  // Event handlers
  private messageHandlers: Array<(chunk: StreamingChunk) => void> = []
  private errorHandlers: Array<(error: string) => void> = []
  private connectHandlers: Array<() => void> = []
  private disconnectHandlers: Array<(reason?: string) => void> = []

  async connect(url: string): Promise<void> {
    this.url = url

    return new Promise((resolve, reject) => {
      this.socket = io(url, {
        transports: ['websocket'],
        timeout: 10000,
        autoConnect: true,
      })

      this.socket.on('connect', () => {
        console.log('WebSocket connected')
        this.reconnectAttempts = 0
        this.connectHandlers.forEach(handler => handler())
        resolve()
      })

      this.socket.on('connect_error', error => {
        console.error('WebSocket connection error:', error)
        this.handleReconnect()
        reject(new Error(`WebSocket connection failed: ${error.message}`))
      })

      this.socket.on('disconnect', reason => {
        console.log('WebSocket disconnected:', reason)
        this.disconnectHandlers.forEach(handler => handler(reason))

        if (reason === 'io server disconnect') {
          // Server initiated disconnect, don't reconnect
          return
        }

        this.handleReconnect()
      })

      this.socket.on('stream_chunk', (data: unknown) => {
        try {
          const chunk = StreamingChunkSchema.parse(data)
          this.messageHandlers.forEach(handler => handler(chunk))
        } catch (error) {
          console.error('Invalid streaming chunk received:', error)
          this.errorHandlers.forEach(handler =>
            handler('Invalid streaming chunk format')
          )
        }
      })

      this.socket.on('error', (error: string) => {
        console.error('WebSocket error:', error)
        this.errorHandlers.forEach(handler => handler(error))
      })

      this.socket.on('session_update', (data: unknown) => {
        // Handle session updates
        console.log('Session update received:', data)
      })
    })
  }

  disconnect(): void {
    if (this.socket) {
      this.socket.disconnect()
      this.socket = undefined
    }
    this.reconnectAttempts = 0
  }

  send(event: WebSocketEvent): void {
    if (!this.socket?.connected) {
      throw new Error('WebSocket not connected')
    }

    this.socket.emit(event.type, event)
  }

  onMessage(callback: (chunk: StreamingChunk) => void): void {
    this.messageHandlers.push(callback)
  }

  onError(callback: (error: string) => void): void {
    this.errorHandlers.push(callback)
  }

  onConnect(callback: () => void): void {
    this.connectHandlers.push(callback)
  }

  onDisconnect(callback: (reason?: string) => void): void {
    this.disconnectHandlers.push(callback)
  }

  isConnected(): boolean {
    return this.socket?.connected ?? false
  }

  private handleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('Max reconnection attempts reached')
      this.errorHandlers.forEach(handler =>
        handler('Max reconnection attempts reached')
      )
      return
    }

    this.reconnectAttempts++
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts - 1)

    console.log(`Attempting to reconnect in ${delay}ms (attempt ${this.reconnectAttempts})`)

    setTimeout(() => {
      if (this.url && !this.isConnected()) {
        this.connect(this.url).catch(error => {
          console.error('Reconnection failed:', error)
        })
      }
    }, delay)
  }
}

export const webSocketClient = new WebSocketClientImpl()