// Keyboard utilities for terminal UI

export interface KeyPress {
  name?: string
  ctrl?: boolean
  meta?: boolean
  shift?: boolean
  sequence?: string
}

export interface KeyBinding {
  key: string
  ctrl?: boolean
  meta?: boolean
  shift?: boolean
  description: string
  action: () => void
}

// Common key combinations
export const KEYS = {
  ENTER: '\r',
  TAB: '\t',
  ESCAPE: '\u001b',
  BACKSPACE: '\u007f',
  DELETE: '\u001b[3~',
  UP: '\u001b[A',
  DOWN: '\u001b[B',
  RIGHT: '\u001b[C',
  LEFT: '\u001b[D',
  HOME: '\u001b[H',
  END: '\u001b[F',
  PAGE_UP: '\u001b[5~',
  PAGE_DOWN: '\u001b[6~',

  // Control combinations
  CTRL_A: '\u0001',
  CTRL_C: '\u0003',
  CTRL_D: '\u0004',
  CTRL_E: '\u0005',
  CTRL_L: '\u000c',
  CTRL_N: '\u000e',
  CTRL_P: '\u0010',
  CTRL_R: '\u0012',
  CTRL_U: '\u0015',
  CTRL_W: '\u0017',
  CTRL_Z: '\u001a',
} as const

export function parseKeyPress(input: string): KeyPress {
  const key: KeyPress = { sequence: input }

  if (input === KEYS.ENTER) {
    key.name = 'return'
  } else if (input === KEYS.TAB) {
    key.name = 'tab'
  } else if (input === KEYS.ESCAPE) {
    key.name = 'escape'
  } else if (input === KEYS.BACKSPACE) {
    key.name = 'backspace'
  } else if (input.startsWith('\u001b[')) {
    // ANSI escape sequences
    if (input === KEYS.UP) {
      key.name = 'up'
    } else if (input === KEYS.DOWN) {
      key.name = 'down'
    } else if (input === KEYS.RIGHT) {
      key.name = 'right'
    } else if (input === KEYS.LEFT) {
      key.name = 'left'
    } else if (input === KEYS.HOME) {
      key.name = 'home'
    } else if (input === KEYS.END) {
      key.name = 'end'
    } else if (input === KEYS.DELETE) {
      key.name = 'delete'
    } else if (input === KEYS.PAGE_UP) {
      key.name = 'pageup'
    } else if (input === KEYS.PAGE_DOWN) {
      key.name = 'pagedown'
    }
  } else if (input.charCodeAt(0) < 32) {
    // Control characters
    key.ctrl = true
    key.name = String.fromCharCode(input.charCodeAt(0) + 96) // Convert to letter
  } else {
    // Regular character
    key.name = input
  }

  return key
}

export function keyMatches(keyPress: KeyPress, binding: KeyBinding): boolean {
  return (
    keyPress.name === binding.key &&
    !!keyPress.ctrl === !!binding.ctrl &&
    !!keyPress.meta === !!binding.meta &&
    !!keyPress.shift === !!binding.shift
  )
}

export function formatKeyBinding(binding: KeyBinding): string {
  const parts: string[] = []

  if (binding.ctrl) parts.push('Ctrl')
  if (binding.meta) parts.push('Meta')
  if (binding.shift) parts.push('Shift')

  parts.push(binding.key.toUpperCase())

  return parts.join('+')
}

// Default key bindings for the application
export const DEFAULT_KEY_BINDINGS: KeyBinding[] = [
  {
    key: 'c',
    ctrl: true,
    description: 'Exit application',
    action: () => process.exit(0),
  },
  {
    key: 'l',
    ctrl: true,
    description: 'Clear screen',
    action: () => {
      // Will be implemented in the UI component
    },
  },
  {
    key: 'r',
    ctrl: true,
    description: 'Refresh/reload',
    action: () => {
      // Will be implemented in the UI component
    },
  },
  {
    key: 'n',
    ctrl: true,
    description: 'New session',
    action: () => {
      // Will be implemented in the UI component
    },
  },
  {
    key: 's',
    ctrl: true,
    description: 'Save session',
    action: () => {
      // Will be implemented in the UI component
    },
  },
]

export class KeyboardHandler {
  private bindings: Map<string, KeyBinding> = new Map()

  constructor(bindings: KeyBinding[] = DEFAULT_KEY_BINDINGS) {
    this.setBindings(bindings)
  }

  setBindings(bindings: KeyBinding[]): void {
    this.bindings.clear()
    for (const binding of bindings) {
      const key = this.getBindingKey(binding)
      this.bindings.set(key, binding)
    }
  }

  addBinding(binding: KeyBinding): void {
    const key = this.getBindingKey(binding)
    this.bindings.set(key, binding)
  }

  removeBinding(binding: Partial<KeyBinding>): void {
    const key = this.getBindingKey(binding as KeyBinding)
    this.bindings.delete(key)
  }

  handleKeyPress(keyPress: KeyPress): boolean {
    for (const binding of this.bindings.values()) {
      if (keyMatches(keyPress, binding)) {
        binding.action()
        return true
      }
    }
    return false
  }

  getBindings(): KeyBinding[] {
    return Array.from(this.bindings.values())
  }

  private getBindingKey(binding: KeyBinding): string {
    return `${binding.key}:${!!binding.ctrl}:${!!binding.meta}:${!!binding.shift}`
  }
}