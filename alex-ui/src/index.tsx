#!/usr/bin/env node

// @ts-ignore - React import needed for JSX
import React from 'react'
import { render } from 'ink'
import { Command } from 'commander'
import { App } from './app/App'

interface CLIOptions {
  apiUrl?: string
  wsUrl?: string
  noWebsocket?: boolean
  debug?: boolean
  version?: boolean
}

function createCLI() {
  const program = new Command()

  program
    .name('alex-ui')
    .description('ALEX - AI Code Agent Terminal UI')
    .version('0.1.0')
    .option('-a, --api-url <url>', 'API server URL', 'http://localhost:8080/api')
    .option('-w, --ws-url <url>', 'WebSocket server URL', 'ws://localhost:8080/api/sessions')
    .option('--no-websocket', 'Disable WebSocket connection')
    .option('-d, --debug', 'Enable debug mode')
    .option('-v, --version', 'Show version')

  return program
}

function main() {
  const program = createCLI()
  const options = program.parse().opts<CLIOptions>()

  if (options.debug) {
    console.log('Debug mode enabled')
    console.log('Options:', options)
  }

  const config = {
    apiUrl: options.apiUrl,
    wsUrl: options.wsUrl,
    enableWebSocket: !options.noWebsocket,
  }

  // Render the React app
  const { unmount } = render(<App config={config} />)

  // Handle process signals
  process.on('SIGINT', () => {
    unmount()
    process.exit(0)
  })

  process.on('SIGTERM', () => {
    unmount()
    process.exit(0)
  })

  // Handle uncaught exceptions
  process.on('uncaughtException', error => {
    console.error('Uncaught exception:', error)
    unmount()
    process.exit(1)
  })

  process.on('unhandledRejection', (reason, promise) => {
    console.error('Unhandled rejection at:', promise, 'reason:', reason)
    unmount()
    process.exit(1)
  })
}

// Run main function for CLI execution
main()

export { App }
export default main