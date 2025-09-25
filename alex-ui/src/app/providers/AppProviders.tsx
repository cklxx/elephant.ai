import React from 'react'
import { ErrorBoundary } from '@/components/common'
import { ConnectionProvider } from './ConnectionProvider'

export interface AppProvidersProps {
  children: React.ReactNode
  config?: {
    apiUrl?: string
    wsUrl?: string
    enableWebSocket?: boolean
  }
}

export const AppProviders: React.FC<AppProvidersProps> = ({
  children,
  config = {},
}) => {
  return (
    <ErrorBoundary>
      <ConnectionProvider
        apiUrl={config.apiUrl}
        wsUrl={config.wsUrl}
        enableWebSocket={config.enableWebSocket}
      >
        {children}
      </ConnectionProvider>
    </ErrorBoundary>
  )
}