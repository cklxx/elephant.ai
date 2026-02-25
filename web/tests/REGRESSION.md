# Visual Regression Testing Workflow

## Overview

Visual regression testing ensures UI changes are intentional and don't introduce unexpected visual bugs. We use Storybook + Chromatic for automated visual testing.

## Tools

- **Storybook**: Component development environment
- **Chromatic**: Visual regression testing service
- **Playwright**: E2E screenshot comparisons

## Storybook Setup

### Running Storybook Locally

```bash
# Start Storybook dev server
npm run storybook

# Build static Storybook
npm run build-storybook
```

Visit `http://localhost:6006` to view stories.

## Visual Testing Workflow

### 1. Create Component Stories

For each component, create a `.stories.tsx` file:

```typescript
// components/agent/TaskInput.stories.tsx
import type { Meta, StoryObj } from '@storybook/react';
import { TaskInput } from './TaskInput';

const meta: Meta<typeof TaskInput> = {
  title: 'Agent/TaskInput',
  component: TaskInput,
  tags: ['autodocs'],
};

export default meta;
type Story = StoryObj<typeof TaskInput>;

// Default state
export const Default: Story = {
  args: {},
};

// Loading state
export const Loading: Story = {
  args: {
    isLoading: true,
  },
};

// Error state
export const WithError: Story = {
  args: {
    error: 'Something went wrong',
  },
};
```

### 2. Cover All Component States

For comprehensive visual testing, create stories for:

- **Default state**: Component with no props
- **Loading state**: While data is fetching
- **Error state**: When an error occurs
- **Empty state**: No data to display
- **Populated state**: With typical data
- **Edge cases**: Long text, many items, etc.
- **Interactive states**: Hover, focus, active, disabled

### 3. Capture Baselines

First run captures baseline snapshots:

```bash
npx chromatic --project-token=<your-token>
```

This creates a baseline for future comparisons.

### 4. Review Changes

On subsequent runs, Chromatic:
1. Captures new snapshots
2. Compares against baselines
3. Highlights visual differences
4. Requires manual approval/rejection

## Component Coverage Checklist

### Agent Components

- [x] TaskInput
  - [x] Default
  - [x] Loading
  - [x] Disabled
  - [x] With placeholder
- [x] ConnectionStatus
  - [x] Connected
  - [x] Offline
  - [x] Reconnecting
  - [x] Max retries
- [x] ThinkingIndicator
  - [x] Default
  - [x] In card
  - [x] Dark background
- [ ] ToolCallCard
  - [ ] Running
  - [ ] Streaming
  - [ ] Complete
  - [ ] Error
  - [ ] Expanded/Collapsed
- [ ] TaskAnalysisCard
  - [ ] Default
  - [ ] With long text
- [ ] TaskCompleteCard
  - [ ] Success
  - [ ] With statistics
- [ ] ErrorCard
  - [ ] Recoverable
  - [ ] Non-recoverable
- [ ] AgentOutput
  - [ ] Empty
  - [ ] With events
  - [ ] Scrollable

### Session Components

- [ ] SessionCard
  - [ ] Default
  - [ ] With actions
  - [ ] Active
  - [ ] Inactive
- [ ] SessionList
  - [ ] Empty
  - [ ] Loading
  - [ ] With sessions
  - [ ] Grid layout

### UI Components

- [ ] Button
  - [ ] All variants (primary, secondary, outline, ghost)
  - [ ] All sizes (sm, md, lg)
  - [ ] Disabled
  - [ ] Loading
- [ ] Card
  - [ ] Default
  - [ ] With header/footer
  - [ ] Hoverable
- [ ] Badge
  - [ ] All variants (default, success, warning, error, info)

## Theme Coverage

Test all components in:

- [x] Light theme
- [ ] Dark theme
- [ ] High contrast mode

## Viewport Coverage

Test responsive behavior at:

- [x] Mobile (375px)
- [x] Tablet (768px)
- [x] Desktop (1440px)

## Chromatic Configuration

### Project Setup

1. Sign up at [chromatic.com](https://www.chromatic.com/)
2. Create a new project
3. Link to GitHub repository
4. Get project token

### Environment Variables

```bash
# .env.local
CHROMATIC_PROJECT_TOKEN=your-token-here
```

### CI Integration

Add to `.github/workflows/chromatic.yml`:

```yaml
name: Chromatic

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  chromatic-deployment:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0

      - name: Setup Node
        uses: actions/setup-node@v3
        with:
          node-version: 18

      - name: Install dependencies
        run: npm ci

      - name: Run Chromatic
        uses: chromaui/action@v1
        with:
          projectToken: ${{ secrets.CHROMATIC_PROJECT_TOKEN }}
          exitZeroOnChanges: true
```

## Local Screenshot Testing

For quick visual checks without Chromatic:

```bash
# Using Playwright
npx playwright test --update-snapshots
```

## Visual Testing Best Practices

### 1. Consistent Data

Use fixed data in stories to avoid flaky tests:

```typescript
export const WithData: Story = {
  args: {
    timestamp: '2025-10-05T12:00:00Z', // Fixed timestamp
    user: 'Test User', // Fixed name
  },
};
```

### 2. Disable Animations

Prevent flaky screenshots:

```typescript
export const Default: Story = {
  parameters: {
    chromatic: {
      disableSnapshot: false,
      delay: 300, // Wait for animations
    },
  },
};
```

### 3. Ignore Dynamic Elements

Exclude timestamps, random IDs:

```typescript
export const WithTimestamp: Story = {
  parameters: {
    chromatic: {
      ignore: ['.timestamp-element'],
    },
  },
};
```

### 4. Test Interactions

Capture component after user interaction:

```typescript
import { within, userEvent } from '@storybook/testing-library';

export const Focused: Story = {
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const input = canvas.getByRole('textbox');
    await userEvent.click(input);
  },
};
```

## Review Process

### When Visual Changes Are Detected

1. **Review in Chromatic**
   - Click "Review changes" in PR
   - Compare before/after screenshots
   - Check all viewports

2. **Approve or Reject**
   - ✅ Approve if change is intentional
   - ❌ Reject if change is a bug
   - 💬 Comment for clarification

3. **Update Baselines**
   - Approved changes become new baselines
   - Future tests compare against new baseline

### False Positives

Common causes:
- Font rendering differences
- Timing issues (animations)
- Random data
- External resources (images not loaded)

Solutions:
- Use deterministic data
- Disable animations
- Add delays
- Mock external resources

## Performance Considerations

### Optimize Snapshot Count

- Don't snapshot every possible state
- Focus on distinct visual states
- Combine similar variations

### Snapshot Size

- Chromatic has snapshot limits
- Use `disableSnapshot: true` for non-visual stories
- Split large component sets across multiple projects

## Accessibility Integration

Storybook includes a11y addon:

```bash
npm install --save-dev @storybook/addon-a11y
```

Add to `.storybook/main.ts`:

```typescript
addons: [
  '@storybook/addon-a11y',
],
```

Run accessibility checks alongside visual tests.

## Reporting

### Coverage Report

Track visual test coverage:

```bash
# Count stories
find . -name "*.stories.tsx" | wc -l

# Count components
find components -name "*.tsx" | grep -v stories | wc -l
```

Calculate coverage:
```
Coverage = (Stories / Components) * 100
```

### Visual Diff History

Chromatic provides:
- Diff history for each component
- Timeline of visual changes
- Blame information (who changed what)

## Troubleshooting

### Stories Not Appearing

1. Check file naming: `*.stories.tsx`
2. Verify export: `export default meta`
3. Check Storybook config paths
4. Restart Storybook server

### Chromatic Failing

1. Check project token
2. Verify internet connection
3. Check build status
4. Review error logs

### Flaky Tests

1. Add delays for animations
2. Use fixed data
3. Disable dynamic content
4. Increase thresholds

## Resources

- [Storybook Documentation](https://storybook.js.org/docs)
- [Chromatic Documentation](https://www.chromatic.com/docs)
- [Visual Testing Best Practices](https://storybook.js.org/docs/writing-tests/visual-testing)
- [Accessibility Testing](https://storybook.js.org/docs/writing-tests/accessibility-testing)
