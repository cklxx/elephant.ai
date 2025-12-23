import { beforeEach, describe, expect, it } from 'vitest';

import { defaultEventRegistry } from '@/lib/events/eventRegistry';
import { useSessionStore } from '@/hooks/useSessionStore';
import { WorkflowToolCompletedEvent } from '@/lib/types';

describe('plan goal session titles', () => {
  beforeEach(() => {
    localStorage.clear();
    useSessionStore.setState({
      currentSessionId: null,
      sessionHistory: [],
      sessionLabels: {},
    });
  });

  it('uses the plan session_title from metadata as the session label', () => {
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

    expect(useSessionStore.getState().sessionLabels['session-plan-1']).toBe(
      'Plan labeling',
    );
  });

  it('falls back to the plan result when no metadata goal is present', () => {
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

    expect(useSessionStore.getState().sessionLabels['session-plan-2']).toBe(
      'Draft new help center IA',
    );
  });

  it('falls back to overall_goal_ui when session_title is missing', () => {
    const event: WorkflowToolCompletedEvent = {
      event_type: 'workflow.tool.completed',
      agent_level: 'core',
      timestamp: new Date().toISOString(),
      session_id: 'session-plan-3',
      call_id: 'call-plan-3',
      tool_name: 'plan',
      result: 'Refine onboarding experience',
      duration: 120,
      metadata: { overall_goal_ui: 'Improve plan labeling UX' },
    };

    defaultEventRegistry.run(event);

    expect(useSessionStore.getState().sessionLabels['session-plan-3']).toBe(
      'Improve plan labeling UX',
    );
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
});
