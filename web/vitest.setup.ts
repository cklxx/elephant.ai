import '@testing-library/jest-dom';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';
import { enableMapSet } from 'immer';

// Enable Immer MapSet plugin for Zustand tests
enableMapSet();

class MemoryStorage implements Storage {
  #store = new Map<string, string>();

  get length() {
    return this.#store.size;
  }

  clear() {
    this.#store.clear();
  }

  getItem(key: string) {
    return this.#store.has(key) ? this.#store.get(key)! : null;
  }

  key(index: number) {
    if (index < 0 || index >= this.#store.size) return null;
    return Array.from(this.#store.keys())[index] ?? null;
  }

  removeItem(key: string) {
    this.#store.delete(key);
  }

  setItem(key: string, value: string) {
    this.#store.set(key, String(value));
  }
}

function ensureStorage(name: 'localStorage' | 'sessionStorage') {
  const storage = (window as any)[name];
  if (!storage || typeof storage.getItem !== 'function' || typeof storage.setItem !== 'function' || typeof storage.clear !== 'function') {
    Object.defineProperty(window, name, {
      value: new MemoryStorage(),
      configurable: true,
      writable: true,
    });
  }
}

ensureStorage('localStorage');
ensureStorage('sessionStorage');

// Mock PostHog client to avoid network calls in tests
vi.mock('posthog-js', () => {
  const posthogMock: any = {
    capture: vi.fn(),
    identify: vi.fn(),
    register: vi.fn(),
    reset: vi.fn(),
    shutdown: vi.fn(),
    on: vi.fn((_, callback) => {
      if (typeof callback === 'function') {
        callback(posthogMock);
      }
    }),
  };
  posthogMock.people = { set: vi.fn() };
  posthogMock.init = vi.fn((_key: string, options?: { loaded?: (client: any) => void }) => {
    options?.loaded?.(posthogMock);
    return posthogMock;
  });
  return { default: posthogMock };
});

// Cleanup after each test
afterEach(() => {
  cleanup();
});

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

// Mock IntersectionObserver
global.IntersectionObserver = class IntersectionObserver {
  constructor() {}
  disconnect() {}
  observe() {}
  takeRecords() {
    return [];
  }
  unobserve() {}
} as any;

// Mock ResizeObserver
global.ResizeObserver = class ResizeObserver {
  constructor() {}
  disconnect() {}
  observe() {}
  unobserve() {}
} as any;

// Mutate the existing console so happy-dom keeps the mocked methods.
Object.assign(console, {
  log: vi.fn(),
  debug: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
});

const defaultFetch = vi.fn(async (input: RequestInfo | URL) => {
  const url = typeof input === "string" ? input : input instanceof URL ? input.toString() : input.url;
  return {
    ok: true,
    status: 204,
    statusText: "No Content",
    url,
    headers: new Headers(),
    text: async () => "",
    json: async () => ({}),
    arrayBuffer: async () => new ArrayBuffer(0),
    blob: async () => new Blob([]),
  } as Response;
});

Object.defineProperty(globalThis, "fetch", {
  value: defaultFetch,
  writable: true,
  configurable: true,
});
