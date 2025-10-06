# ALEX Web UI Testing Documentation

## Overview

This directory contains comprehensive testing infrastructure for the ALEX Web UI, including unit tests, integration tests, E2E tests, and visual regression tests.

## Testing Stack

### Unit & Integration Tests
- **Framework**: Vitest
- **Testing Library**: @testing-library/react
- **Environment**: jsdom / happy-dom

### E2E Tests
- **Framework**: Playwright
- **Browsers**: Chromium, Firefox, WebKit
- **Viewports**: Desktop, Tablet, Mobile

### Visual Regression
- **Framework**: Storybook + Chromatic
- **Coverage**: All UI components with multiple states

### Performance Testing
- **Tool**: Custom benchmarking scripts
- **Metrics**: Bundle size, TTI, FPS, Memory usage

## Quick Start

```bash
# Install dependencies
npm install

# Run all unit tests
npm test

# Run tests in watch mode
npm run test:watch

# Run tests with coverage
npm run test:coverage

# Run tests with UI
npm run test:ui

# Run E2E tests
npx playwright test

# Run E2E tests with UI
npx playwright test --ui

# Run Storybook
npm run storybook

# Run performance benchmarks
npm run perf-benchmark
```

## Test Structure

```
web/
├── hooks/
│   └── __tests__/
│       ├── useSSE.test.tsx              # SSE connection tests
│       ├── useAgentStreamStore.test.ts   # Event store tests
│       ├── useSessionStore.test.ts       # Session management tests
│       └── useTaskExecution.test.ts      # Task execution tests
│
├── components/
│   ├── agent/
│   │   ├── *.stories.tsx                 # Storybook stories
│   │   └── __tests__/                    # Component tests
│   ├── session/
│   │   ├── *.stories.tsx
│   │   └── __tests__/
│   └── ui/
│       ├── *.stories.tsx
│       └── __tests__/
│
├── e2e/
│   ├── basic-flow.spec.ts               # Basic user flows
│   └── full-user-journey.spec.ts        # Complete scenarios
│
├── scripts/
│   └── perf-benchmark.ts                # Performance testing
│
└── tests/
    ├── README.md                         # This file
    ├── COVERAGE.md                       # Coverage reports
    └── REGRESSION.md                     # Visual regression workflow
```

## Writing Tests

### Unit Tests (Hooks)

```typescript
import { renderHook, act } from '@testing-library/react';
import { useSSE } from '../useSSE';

test('should connect to SSE endpoint', () => {
  const { result } = renderHook(() => useSSE('session-123'));

  act(() => {
    // Simulate connection
  });

  expect(result.current.isConnected).toBe(true);
});
```

### Component Tests

```typescript
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TaskInput } from './TaskInput';

test('should submit task on Enter', async () => {
  const onSubmit = vi.fn();
  render(<TaskInput onSubmit={onSubmit} />);

  const input = screen.getByRole('textbox');
  await userEvent.type(input, 'Test task{Enter}');

  expect(onSubmit).toHaveBeenCalledWith('Test task');
});
```

### E2E Tests

```typescript
import { test, expect } from '@playwright/test';

test('should complete full task flow', async ({ page }) => {
  await page.goto('/');

  await page.getByRole('textbox').fill('Test task');
  await page.getByRole('button', { name: /submit/i }).click();

  await expect(page.getByText(/connected/i)).toBeVisible();
});
```

### Storybook Stories

```typescript
import type { Meta, StoryObj } from '@storybook/react';
import { TaskInput } from './TaskInput';

const meta: Meta<typeof TaskInput> = {
  title: 'Agent/TaskInput',
  component: TaskInput,
};

export default meta;

export const Default: StoryObj<typeof TaskInput> = {
  args: {},
};

export const Loading: StoryObj<typeof TaskInput> = {
  args: {
    isLoading: true,
  },
};
```

## Test Coverage Goals

| Area | Target | Current |
|------|--------|---------|
| Hooks | ≥80% | TBD |
| Components | ≥80% | TBD |
| Utils | ≥80% | TBD |
| Overall | ≥80% | TBD |

## Best Practices

### 1. Test Naming
- Use descriptive test names: `should [expected behavior] when [condition]`
- Group related tests with `describe()` blocks

### 2. Test Isolation
- Each test should be independent
- Use `beforeEach()` to reset state
- Clean up after tests (timers, mocks, etc.)

### 3. Test Coverage
- Test happy paths AND edge cases
- Test error handling
- Test loading states
- Test accessibility

### 4. Mocking
- Mock external dependencies (API calls, EventSource)
- Use Jest/Vitest mocks for modules
- Keep mocks simple and focused

### 5. Assertions
- Use specific matchers (`toBe`, `toEqual`, `toContain`)
- Test observable behavior, not implementation
- Avoid testing internal state

## Common Patterns

### Testing Async Hooks

```typescript
import { waitFor } from '@testing-library/react';

test('should load data', async () => {
  const { result } = renderHook(() => useData());

  await waitFor(() => {
    expect(result.current.isLoading).toBe(false);
  });

  expect(result.current.data).toBeDefined();
});
```

### Testing EventSource

```typescript
class MockEventSource {
  simulateEvent(type: string, data: any) {
    const event = new MessageEvent(type, {
      data: JSON.stringify(data),
    });
    this.listeners.get(type)?.forEach((fn) => fn(event));
  }
}
```

### Testing Zustand Stores

```typescript
beforeEach(() => {
  const { result } = renderHook(() => useStore());
  act(() => {
    result.current.clearState();
  });
});
```

## Continuous Integration

Tests run automatically on:
- Pull requests
- Commits to main branch
- Scheduled daily builds

CI Pipeline:
1. Lint code
2. Run unit tests with coverage
3. Run E2E tests
4. Run performance benchmarks
5. Upload coverage reports
6. Generate test reports

## Debugging Tests

### Debug Individual Test
```bash
# Vitest
npm test -- -t "test name"

# Playwright
npx playwright test --debug
```

### Debug with UI
```bash
# Vitest UI
npm run test:ui

# Playwright UI
npx playwright test --ui
```

### View Test Reports
```bash
# Vitest coverage
open coverage/index.html

# Playwright report
npx playwright show-report
```

## Performance Testing

Run performance benchmarks:

```bash
npm run build
npm run perf-benchmark
```

Metrics measured:
- Bundle size (gzipped)
- Build time
- Runtime performance (planned)
- Memory usage (planned)

## Visual Regression Testing

1. Create Storybook stories for all components
2. Run Chromatic for visual diffs
3. Review and approve changes
4. Baseline snapshots updated automatically

```bash
# Build Storybook
npm run build-storybook

# Run Chromatic (requires token)
npx chromatic --project-token=<token>
```

## Troubleshooting

### Tests Failing Locally

1. Clear test cache: `npm test -- --clearCache`
2. Reinstall dependencies: `rm -rf node_modules package-lock.json && npm install`
3. Check Node version: `node --version` (requires v18+)

### E2E Tests Failing

1. Install Playwright browsers: `npx playwright install`
2. Check dev server is running: `npm run dev`
3. Verify backend is available (if needed)

### Coverage Not Updating

1. Delete coverage directory: `rm -rf coverage`
2. Run fresh coverage: `npm run test:coverage`
3. Check `.gitignore` includes `coverage/`

## Contributing

When adding new features:

1. Write tests FIRST (TDD approach)
2. Ensure all tests pass locally
3. Add Storybook stories for new components
4. Update this documentation if needed
5. Verify coverage meets minimums

## Resources

- [Vitest Documentation](https://vitest.dev)
- [Testing Library](https://testing-library.com)
- [Playwright](https://playwright.dev)
- [Storybook](https://storybook.js.org)
- [Chromatic](https://www.chromatic.com)

## Support

For questions or issues:
1. Check this documentation
2. Review existing tests for patterns
3. Open an issue on GitHub
4. Ask in team chat
