import { act, renderHook } from '@testing-library/react';
import { handleEnvironmentSnapshot, resetDiagnostics, useDiagnostics } from '../useDiagnostics';
import { WorkflowDiagnosticEnvironmentSnapshotEvent } from '@/lib/types';

describe('useDiagnostics', () => {
  beforeEach(() => {
    resetDiagnostics();
  });

  it('returns null environments by default', () => {
    const { result } = renderHook(() => useDiagnostics());
    expect(result.current.environments).toBeNull();
  });

  it('updates environments when snapshot event is handled', () => {
    const event: WorkflowDiagnosticEnvironmentSnapshotEvent = {
      event_type: 'workflow.diagnostic.environment_snapshot',
      timestamp: new Date().toISOString(),
      agent_level: 'core',
      captured: new Date().toISOString(),
      host: { HOSTNAME: 'host.local', USER: 'cli' },
      sandbox: { HOSTNAME: 'sandbox.local', USER: 'runner' },
    };

    const { result } = renderHook(() => useDiagnostics());

    act(() => {
      handleEnvironmentSnapshot(event);
    });

    expect(result.current.environments).not.toBeNull();
    expect(result.current.environments?.host).toMatchObject({ HOSTNAME: 'host.local' });
    expect(result.current.environments?.sandbox).toMatchObject({ USER: 'runner' });
    expect(result.current.environments?.captured).toBe(event.captured);
  });
});
