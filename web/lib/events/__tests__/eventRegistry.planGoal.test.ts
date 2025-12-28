import { beforeEach, describe, expect, it, vi } from 'vitest';

import { defaultEventRegistry } from '@/lib/events/eventRegistry';
import { useSessionStore } from '@/hooks/useSessionStore';
import { WorkflowToolCompletedEvent } from '@/lib/types';
import { apiClient } from '@/lib/api';

vi.mock('@/lib/api', () => ({
  apiClient: {
    getSessionTitle: vi.fn(),
  },
}));

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0));

describe('plan goal session titles', () => {
  beforeEach(() => {
    localStorage.clear();
    useSessionStore.setState({
      currentSessionId: null,
      sessionHistory: [],
      sessionLabels: {},
    });
    vi.mocked(apiClient.getSessionTitle).mockReset();
  });

  it('uses the stored session title as the session label', async () => {
    vi.mocked(apiClient.getSessionTitle).mockResolvedValueOnce('Plan labeling');
    const event: WorkflowToolCompletedEvent = {
      event_type: 'workflow.tool.completed',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-plan-1',
      call_id: 'call-plan',
      tool_name: 'plan',
      result: 'Refine onboarding experience',
      duration: 120,
      metadata: { session_title: 'Plan labeling', overall_goal_ui: 'Improve plan labeling UX' },
    };

    defaultEventRegistry.run(event);
    await flushPromises();

    expect(useSessionStore.getState().sessionLabels['session-plan-1']).toBe(
      'Plan labeling',
    );
  });

  it('skips renaming when no session title is available', async () => {
    vi.mocked(apiClient.getSessionTitle).mockResolvedValueOnce(null);
    const event: WorkflowToolCompletedEvent = {
      event_type: 'workflow.tool.completed',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-plan-2',
      call_id: 'call-plan-2',
      tool_name: 'plan',
      result: 'Draft new help center IA',
      duration: 45,
    };

    defaultEventRegistry.run(event);
    await flushPromises();

    expect(useSessionStore.getState().sessionLabels['session-plan-2']).toBeUndefined();
  });

  it('ignores non-plan tool completions', () => {
    const event: WorkflowToolCompletedEvent = {
      event_type: 'workflow.tool.completed',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-non-plan',
      call_id: 'call-non-plan',
      tool_name: 'search',
      result: 'Unrelated output',
      duration: 30,
      metadata: { overall_goal_ui: 'Should not be used' },
    };

    defaultEventRegistry.run(event);

    expect(useSessionStore.getState().sessionLabels['session-non-plan']).toBeUndefined();
  });

  it('does not override an existing session label', async () => {
    useSessionStore.setState({
      sessionLabels: { 'session-plan-3': 'Existing label' },
    });
    vi.mocked(apiClient.getSessionTitle).mockResolvedValueOnce('New label');

    const event: WorkflowToolCompletedEvent = {
      event_type: 'workflow.tool.completed',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-plan-3',
      call_id: 'call-plan-3',
      tool_name: 'plan',
      result: 'Refine onboarding experience',
      duration: 120,
    };

    defaultEventRegistry.run(event);
    await flushPromises();

    expect(useSessionStore.getState().sessionLabels['session-plan-3']).toBe(
      'Existing label',
    );
    expect(apiClient.getSessionTitle).not.toHaveBeenCalled();
  });
});
