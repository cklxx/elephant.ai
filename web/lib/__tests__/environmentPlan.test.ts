import { describe, it, expect } from 'vitest';
import {
  buildEnvironmentPlan,
  formatEnvironmentPlanShareText,
  serializeEnvironmentPlan,
} from '../environmentPlan';
import { ToolCallSummary } from '../eventAggregation';

describe('buildEnvironmentPlan', () => {
  it('marks sandbox as required when tools demand it', () => {
    const summaries: ToolCallSummary[] = [
      {
        callId: 'call-1',
        toolName: 'code_execute',
        status: 'completed',
        startedAt: '2025-01-01T10:00:00Z',
        completedAt: '2025-01-01T10:00:05Z',
        durationMs: 5000,
        argumentsPreview: 'language: python',
        resultPreview: 'output',
        errorMessage: undefined,
        requiresSandbox: true,
        sandboxLevel: 'system',
      },
    ];

    const plan = buildEnvironmentPlan('session-1', summaries);
    expect(plan.sessionId).toBe('session-1');
    expect(plan.sandboxStrategy).toBe('required');
    expect(plan.toolsUsed).toEqual(['code_execute']);
    expect(plan.blueprint.recommendedCapabilities).toContain('sandbox-enforced');
    expect(plan.notes).toContain('All tool calls are sandboxed');
    expect(plan.todos[0]).toMatchObject({ id: 'confirm-sandbox', completed: true });
    expect(plan.todos.some((todo) => todo.id === 'route-sandbox-tools')).toBe(true);
  });

  it('defaults to recommended sandbox when no tool requires it', () => {
    const summaries: ToolCallSummary[] = [];

    const plan = buildEnvironmentPlan('session-2', summaries);
    expect(plan.sandboxStrategy).toBe('recommended');
    expect(plan.blueprint.recommendedCapabilities).toContain('network-isolation');
    expect(plan.toolsUsed).toEqual([]);
    expect(
      plan.todos.some((todo) => todo.id === 'await-first-call' && !todo.completed)
    ).toBe(true);
  });

  it('adds todo items for running and error tool states', () => {
    const summaries: ToolCallSummary[] = [
      {
        callId: 'call-running',
        toolName: 'shell_exec',
        status: 'running',
        startedAt: '2025-01-01T10:00:00Z',
        requiresSandbox: true,
        sandboxLevel: 'system',
      } as ToolCallSummary,
      {
        callId: 'call-error',
        toolName: 'file_writer',
        status: 'error',
        startedAt: '2025-01-01T11:00:00Z',
        requiresSandbox: true,
        errorMessage: 'boom',
        sandboxLevel: 'filesystem',
      } as ToolCallSummary,
    ];

    const plan = buildEnvironmentPlan('session-3', summaries);

    expect(
      plan.todos.some(
        (todo) => todo.id === 'monitor-running' && todo.label.includes('shell_exec')
      )
    ).toBe(true);
    expect(
      plan.todos.some(
        (todo) => todo.id === 'inspect-failures' && todo.label.includes('file_writer')
      )
    ).toBe(true);
  });

  it('marks blueprint todo as complete when a previous plan exists', () => {
    const summaries: ToolCallSummary[] = [];
    const initialPlan = buildEnvironmentPlan('session-4', summaries);

    expect(
      initialPlan.todos.find((todo) => todo.id === 'persist-blueprint')?.completed
    ).toBe(false);

    const updatedPlan = buildEnvironmentPlan('session-4', summaries, initialPlan);

    expect(
      updatedPlan.todos.find((todo) => todo.id === 'persist-blueprint')?.completed
    ).toBe(true);
  });
});

describe('formatEnvironmentPlanShareText', () => {
  it('produces a readable checklist with sandbox details', () => {
    const summaries: ToolCallSummary[] = [
      {
        callId: 'c1',
        toolName: 'shell_exec',
        status: 'completed',
        startedAt: '2025-01-02T00:00:00Z',
        completedAt: '2025-01-02T00:00:10Z',
        durationMs: 10000,
        requiresSandbox: true,
        sandboxLevel: 'system',
      } as ToolCallSummary,
    ];

    const plan = buildEnvironmentPlan('session-share', summaries);
    const text = formatEnvironmentPlanShareText(plan);

    expect(text).toContain('Sandbox plan for session session-share');
    expect(text).toContain('shell_exec');
    expect(text).toContain('Todos:');
    expect(text).toContain('- [x]');
  });
});

describe('serializeEnvironmentPlan', () => {
  it('omits tool summaries and keeps essential fields', () => {
    const summaries: ToolCallSummary[] = [
      {
        callId: 'c2',
        toolName: 'file_writer',
        status: 'completed',
        startedAt: '2025-01-03T00:00:00Z',
        completedAt: '2025-01-03T00:00:05Z',
        durationMs: 5000,
        requiresSandbox: true,
        sandboxLevel: 'filesystem',
        resultPreview: 'ok',
      } as ToolCallSummary,
    ];

    const plan = buildEnvironmentPlan('session-export', summaries);
    const serialized = serializeEnvironmentPlan(plan);

    expect(serialized.sessionId).toBe('session-export');
    expect(serialized.blueprint.title).toBeDefined();
    expect(serialized.todos.length).toBeGreaterThan(0);
    expect(serialized as any).not.toHaveProperty('toolSummaries');
  });
});
