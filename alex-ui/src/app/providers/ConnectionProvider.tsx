import React from 'react'
import { useAppStore } from '@/stores'
import { connectionManager } from '@/services'

export interface ConnectionProviderProps {
  children: React.ReactNode
  apiUrl?: string
  wsUrl?: string
  enableWebSocket?: boolean
}

export const ConnectionProvider: React.FC<ConnectionProviderProps> = ({
  children,
  apiUrl,
  wsUrl,
  enableWebSocket = true,
}) => {
  const { setConnected, setConnectionError, setAvailableTools } = useAppStore()

  React.useEffect(() => {
    const initializeConnection = async () => {
      try {
        await connectionManager.initialize({
          apiUrl,
          wsUrl,
          enableWebSocket,
        })

        // Load available tools
        const { apiClient } = await import('@/services')
        const tools = await apiClient.getAvailableTools()
        setAvailableTools(tools)
      } catch (error) {
        console.error('Failed to initialize connection:', error)
        setConnectionError(
          error instanceof Error ? error.message : 'Connection failed'
        )
      }
    }

    initializeConnection()

    // Cleanup on unmount
    return () => {
      connectionManager.shutdown().catch(console.error)
    }
  }, [
    apiUrl,
    wsUrl,
    enableWebSocket,
    setConnected,
    setConnectionError,
    setAvailableTools,
  ])

  return <>{children}</>
}
