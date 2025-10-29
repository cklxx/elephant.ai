import { AnyAgentEvent } from '@/lib/types';

type MockEventPayload = Pick<AnyAgentEvent, 'event_type' | 'agent_level'> & Record<string, unknown>;

export interface TimedMockEvent {
  delay: number;
  event: MockEventPayload;
}

export function createMockEventSequence(task: string): TimedMockEvent[] {
  const safeTask = task || 'Analyze the repository and suggest improvements';
  const callId = 'mock-call-1';

  return [
    {
      delay: 300,
      event: {
        event_type: 'task_analysis',
        agent_level: 'core',
        action_name: 'Understanding task requirements',
        goal: safeTask,
      },
    },
    {
      delay: 650,
      event: {
        event_type: 'research_plan',
        agent_level: 'core',
        plan_steps: [
          'Audit existing project structure',
          'Identify UI inconsistencies',
          'Prepare actionable refactor plan',
        ],
        estimated_iterations: 3,
      },
    },
    {
      delay: 950,
      event: {
        event_type: 'step_started',
        agent_level: 'core',
        step_index: 0,
        step_description: 'Collecting repository context',
      },
    },
    {
      delay: 1200,
      event: {
        event_type: 'iteration_start',
        agent_level: 'core',
        iteration: 1,
        total_iters: 3,
      },
    },
    {
      delay: 1450,
      event: {
        event_type: 'thinking',
        agent_level: 'core',
        iteration: 1,
        message_count: 1,
      },
    },
    {
      delay: 1700,
      event: {
        event_type: 'tool_call_start',
        agent_level: 'core',
        iteration: 1,
        call_id: callId,
        tool_name: 'file_read',
        arguments: {
          path: 'web/app/page.tsx',
        },
      },
    },
    {
      delay: 1950,
      event: {
        event_type: 'tool_call_stream',
        agent_level: 'core',
        call_id: callId,
        chunk: 'Inspecting layout composition...\n',
        is_complete: false,
      },
    },
    {
      delay: 2150,
      event: {
        event_type: 'tool_call_stream',
        agent_level: 'core',
        call_id: callId,
        chunk: 'Detected conditional input rendering issue.\n',
        is_complete: false,
      },
    },
    {
      delay: 2350,
      event: {
        event_type: 'tool_call_stream',
        agent_level: 'core',
        call_id: callId,
        chunk: 'Preparing remediation suggestions...\n',
        is_complete: true,
      },
    },
    {
      delay: 2550,
      event: {
        event_type: 'tool_call_complete',
        agent_level: 'core',
        call_id: callId,
        tool_name: 'file_read',
        result: 'Successfully reviewed component layout and state management.',
        duration: 420,
      },
    },
    {
      delay: 2750,
      event: {
        event_type: 'browser_info',
        agent_level: 'core',
        success: true,
        message: 'Sandbox browser ready',
        user_agent: 'MockBrowser/1.0',
        cdp_url: 'ws://sandbox.example.com/devtools',
        vnc_url: 'vnc://sandbox.example.com',
        viewport_width: 1280,
        viewport_height: 720,
        captured: '2024-01-01T00:00:00Z',
      },
    },
    {
      delay: 2950,
      event: {
        event_type: 'step_completed',
        agent_level: 'core',
        step_index: 0,
        step_result: 'Collected baseline UI findings',
      },
    },
    {
      delay: 3200,
      event: {
        event_type: 'iteration_complete',
        agent_level: 'core',
        iteration: 1,
        tokens_used: 865,
        tools_run: 1,
      },
    },
    {
      delay: 3450,
      event: {
        event_type: 'think_complete',
        agent_level: 'core',
        iteration: 1,
        content: 'Ready to summarize the refactor recommendations.',
        tool_call_count: 1,
      },
    },
    {
      delay: 3700,
      event: {
        event_type: 'task_complete',
        agent_level: 'core',
        final_answer:
          '1. Keep the input dock always visible.\n2. Stream events in a terminal column.\n3. Provide reconnection affordances with the new console styling.',
        total_iterations: 1,
        total_tokens: 865,
        stop_reason: 'completed',
        duration: 3600,
      },
    },
  ];
}
