# Test Coverage Report

## Current Coverage Status

> **Note**: Run `npm run test:coverage` to generate the latest coverage report.

Last updated: 2025-10-05

## Coverage by Module

### Hooks

| File | Lines | Functions | Branches | Statements |
|------|-------|-----------|----------|------------|
| useSSE.ts | TBD% | TBD% | TBD% | TBD% |
| useAgentStreamStore.ts | TBD% | TBD% | TBD% | TBD% |
| useSessionStore.ts | TBD% | TBD% | TBD% | TBD% |
| useTaskExecution.ts | TBD% | TBD% | TBD% | TBD% |

**Target**: ≥80% for all metrics

### Components

| File | Lines | Functions | Branches | Statements |
|------|-------|-----------|----------|------------|
| TaskInput.tsx | TBD% | TBD% | TBD% | TBD% |
| AgentOutput.tsx | TBD% | TBD% | TBD% | TBD% |
| ConnectionStatus.tsx | TBD% | TBD% | TBD% | TBD% |
| ToolCallCard.tsx | TBD% | TBD% | TBD% | TBD% |
| SessionList.tsx | TBD% | TBD% | TBD% | TBD% |
| SessionCard.tsx | TBD% | TBD% | TBD% | TBD% |

**Target**: ≥80% for all metrics

### Libraries

| File | Lines | Functions | Branches | Statements |
|------|-------|-----------|----------|------------|
| api.ts | TBD% | TBD% | TBD% | TBD% |
| utils.ts | TBD% | TBD% | TBD% | TBD% |
| types.ts | N/A | N/A | N/A | N/A |
| eventAggregation.ts | TBD% | TBD% | TBD% | TBD% |

**Target**: ≥80% for all metrics

## Overall Coverage

| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Lines | TBD% | ≥80% | ⏳ |
| Functions | TBD% | ≥80% | ⏳ |
| Branches | TBD% | ≥80% | ⏳ |
| Statements | TBD% | ≥80% | ⏳ |

## Coverage Gaps

### Critical (Must Fix)

- [ ] TBD - Run initial coverage to identify gaps

### Medium Priority

- [ ] TBD

### Low Priority

- [ ] TBD

## How to Improve Coverage

### 1. Identify Uncovered Code

```bash
npm run test:coverage
open coverage/index.html
```

Browse the HTML report to find:
- Red highlighted lines (not covered)
- Yellow highlighted branches (partially covered)
- Files with low coverage percentages

### 2. Add Missing Tests

Focus on:
- Error handling paths
- Edge cases
- Conditional branches
- Async operations

### 3. Verify Coverage

```bash
# Run coverage check
npm run test:coverage

# Check if thresholds pass
echo $?  # Should be 0 if passed
```

## Coverage Exemptions

Some files are intentionally excluded from coverage:

- **Configuration files**: `*.config.ts`, `*.config.js`
- **Type definitions**: `*.d.ts`
- **Storybook files**: `*.stories.tsx`
- **Build artifacts**: `.next/`, `dist/`
- **Test files**: `**/__tests__/**`, `**/*.test.*`

## Historical Coverage

Track coverage trends over time:

| Date | Lines | Functions | Branches | Statements |
|------|-------|-----------|----------|------------|
| 2025-10-05 | TBD% | TBD% | TBD% | TBD% |
| (Future) | - | - | - | - |

## CI/CD Integration

Coverage reports are:
1. Generated on every PR
2. Uploaded to Codecov (if configured)
3. Commented on PR with delta
4. Block merge if coverage drops below threshold

## Generating Coverage Reports

### Locally

```bash
# Generate coverage
npm run test:coverage

# View HTML report
open coverage/index.html

# View terminal summary
npm run test:coverage -- --reporter=verbose
```

### CI Pipeline

Coverage is automatically generated and reported in CI:

```yaml
# .github/workflows/test.yml
- name: Run tests with coverage
  run: npm run test:coverage

- name: Upload coverage to Codecov
  uses: codecov/codecov-action@v3
  with:
    files: ./coverage/lcov.info
```

## Coverage Configuration

Coverage thresholds are defined in `vitest.config.ts`:

```typescript
coverage: {
  thresholds: {
    lines: 80,
    functions: 80,
    branches: 80,
    statements: 80,
  },
}
```

## Best Practices

1. **Write tests first** (TDD) - ensures coverage from the start
2. **Test behavior, not implementation** - more maintainable tests
3. **Don't chase 100%** - focus on critical paths
4. **Review coverage diffs** - prevent coverage regressions
5. **Document exemptions** - explain why code is uncovered

## Common Coverage Issues

### False Negatives

- Dead code that should be removed
- Defensive checks that can't be triggered
- Type guards in TypeScript

### False Positives

- 100% coverage doesn't mean bug-free
- Can still miss edge cases
- Integration issues may not be caught

## Next Steps

1. Run initial coverage report
2. Identify critical gaps
3. Write tests to fill gaps
4. Update this document with actual numbers
5. Set up automated coverage reporting
6. Monitor coverage in pull requests

## Resources

- [Vitest Coverage](https://vitest.dev/guide/coverage.html)
- [Istanbul Coverage](https://istanbul.js.org/)
- [Codecov](https://about.codecov.io/)
