#!/usr/bin/env node

/**
 * Demo script for ALEX UI
 *
 * This script demonstrates the terminal UI functionality
 * with mock data when the backend is not available.
 */

import React from 'react'
import { render } from 'ink'
import { App } from '../src/app/App'

// Mock backend responses for demo
const mockApiResponses = {
  '/health': { status: 'ok', version: '0.4.6' },
  '/tools': [
    {
      name: 'file_read',
      description: 'Read file contents',
      parameters: { file_path: { type: 'string', description: 'Path to file' } },
      risk_level: 'low'
    },
    {
      name: 'bash',
      description: 'Execute bash commands',
      parameters: { command: { type: 'string', description: 'Command to execute' } },
      risk_level: 'high'
    },
    {
      name: 'grep',
      description: 'Search for patterns in files',
      parameters: { pattern: { type: 'string', description: 'Search pattern' } },
      risk_level: 'low'
    }
  ],
  '/sessions': []
}

// Mock fetch for demo
global.fetch = async (url: string, options?: any) => {
  const path = url.replace(/^.*\/api\/v1/, '')

  if (mockApiResponses[path as keyof typeof mockApiResponses]) {
    return {
      ok: true,
      status: 200,
      json: async () => mockApiResponses[path as keyof typeof mockApiResponses]
    } as Response
  }

  throw new Error(`Mock API: Unknown endpoint ${path}`)
}

console.log('🚀 Starting ALEX UI Demo...')
console.log('📝 This demo runs with mock data (no backend required)')
console.log('⌨️  Try typing some messages and using keyboard shortcuts!')
console.log('')

const demoConfig = {
  apiUrl: 'http://localhost:8080/api/v1',
  wsUrl: 'ws://localhost:8080/ws',
  enableWebSocket: false, // Disable WebSocket for demo
}

render(React.createElement(App, { config: demoConfig }))