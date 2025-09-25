import { beforeEach, vi } from 'vitest'

// Mock WebSocket for tests
global.WebSocket = class MockWebSocket {
  readyState = 1
  send = vi.fn()
  close = vi.fn()
  addEventListener = vi.fn()
  removeEventListener = vi.fn()
  dispatchEvent = vi.fn()
} as any

// Mock fetch for tests
global.fetch = vi.fn()

// Mock crypto.randomUUID
Object.defineProperty(global, 'crypto', {
  value: {
    randomUUID: () => 'mock-uuid-' + Math.random().toString(36).substring(2, 15),
  },
})

beforeEach(() => {
  vi.clearAllMocks()
})