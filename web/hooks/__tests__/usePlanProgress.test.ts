import { describe, it, expect } from 'vitest';
import { renderHook } from '@testing-library/react';
import { usePlanProgress } from '../usePlanProgress';
import { TimelineStep } from '@/lib/planTypes';

describe('usePlanProgress', () => {
  it('returns null when no steps are provided', () => {
    const { result } = renderHook(() => usePlanProgress([]));

    expect(result.current).toBeNull();
  });

  it('computes completion metrics and aggregates unique tools', () => {
    const steps: TimelineStep[] = [
      {
        id: 'step-1',
        title: 'Collect requirements',
        status: 'done',
        duration: 120000,
        startTime: 0,
        endTime: 120000,
        toolsUsed: ['web_fetch', 'file_read'],
        tokensUsed: 1200,
      },
      {
        id: 'step-2',
        title: 'Implement feature',
        status: 'active',
        startTime: 130000,
        toolsUsed: ['file_edit'],
      },
      {
        id: 'step-3',
        title: 'Write tests',
        status: 'planned',
      },
    ];

    const { result } = renderHook(() => usePlanProgress(steps));

    expect(result.current).not.toBeNull();
    expect(result.current?.totalSteps).toBe(3);
    expect(result.current?.completedSteps).toBe(1);
    expect(result.current?.completionRatio).toBeCloseTo(1 / 3);
    expect(result.current?.activeStepId).toBe('step-2');
    expect(result.current?.activeStepTitle).toBe('Implement feature');
    expect(result.current?.latestCompletedStepId).toBe('step-1');
    expect(result.current?.latestCompletedStepTitle).toBe('Collect requirements');
    expect(result.current?.totalDurationMs).toBe(120000);
    expect(result.current?.averageStepDurationMs).toBe(120000);
    expect(result.current?.totalTokensUsed).toBe(1200);
    expect(result.current?.uniqueToolsUsed).toEqual(['file_edit', 'file_read', 'web_fetch']);
    expect(result.current?.hasErrors).toBe(false);
    expect(result.current?.stepStatuses).toEqual({
      'step-1': 'done',
      'step-2': 'active',
      'step-3': 'planned',
    });
  });

  it('flags errors and computes averages from multiple completed steps', () => {
    const steps: TimelineStep[] = [
      {
        id: 'step-1',
        title: 'Run analysis',
        status: 'done',
        duration: 60000,
        startTime: 0,
        endTime: 60000,
        tokensUsed: 800,
      },
      {
        id: 'step-2',
        title: 'Execute plan',
        status: 'failed',
        duration: 90000,
        startTime: 60000,
        endTime: 150000,
        error: 'Tool failed',
      },
      {
        id: 'step-3',
        title: 'Summarize results',
        status: 'done',
        duration: 30000,
        startTime: 150000,
        endTime: 180000,
      },
    ];

    const { result } = renderHook(() => usePlanProgress(steps));

    expect(result.current).not.toBeNull();
    expect(result.current?.completedSteps).toBe(2);
    expect(result.current?.totalDurationMs).toBe(60000 + 90000 + 30000);
    expect(result.current?.averageStepDurationMs).toBeCloseTo((60000 + 90000 + 30000) / 3);
    expect(result.current?.hasErrors).toBe(true);
    expect(result.current?.latestCompletedStepId).toBe('step-3');
    expect(result.current?.latestCompletedStepTitle).toBe('Summarize results');
    expect(result.current?.stepStatuses['step-2']).toBe('failed');
  });
});
