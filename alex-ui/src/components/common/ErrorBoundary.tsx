import React from 'react'
import { Box, Text } from 'ink'

interface ErrorBoundaryState {
  hasError: boolean
  error?: Error
}

export interface ErrorBoundaryProps {
  children: React.ReactNode
  fallback?: React.ComponentType<{ error: Error }>
}

export class ErrorBoundary extends React.Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props)
    this.state = { hasError: false }
  }

  static getDerivedStateFromError(error: Error): ErrorBoundaryState {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('ErrorBoundary caught an error:', error, errorInfo)
  }

  render() {
    if (this.state.hasError) {
      if (this.props.fallback && this.state.error) {
        return <this.props.fallback error={this.state.error} />
      }

      return (
        <Box
          flexDirection="column"
          padding={1}
          borderStyle="round"
          borderColor="red"
        >
          <Text color="red" bold>
            Application Error
          </Text>
          <Text color="red">
            {this.state.error?.message || 'An unexpected error occurred'}
          </Text>
          {this.state.error?.stack && (
            <Box marginTop={1}>
              <Text color="gray">{this.state.error.stack}</Text>
            </Box>
          )}
        </Box>
      )
    }

    return this.props.children
  }
}
