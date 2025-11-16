import { AgentEvent, AnyAgentEvent } from '@/lib/types';

export type MockEventPayload = Partial<AnyAgentEvent> &
  Pick<AgentEvent, 'event_type' | 'agent_level'> &
  Partial<
    Pick<
      AgentEvent,
      'is_subtask' | 'parent_task_id' | 'subtask_index' | 'total_subtasks' | 'subtask_preview' | 'max_parallel'
    >
  > &
  Record<string, unknown>;

export interface TimedMockEvent {
  delay: number;
  event: MockEventPayload;
}

export function createMockEventSequence(task: string): TimedMockEvent[] {
  const safeTask = task || 'Analyze the repository and suggest improvements';
  const callId = 'mock-call-1';
  const parentTaskId = 'mock-core-task';

  const subtaskOneMeta = {
    is_subtask: true,
    parent_task_id: parentTaskId,
    subtask_index: 0,
    total_subtasks: 2,
    subtask_preview: 'Research comparable console UX patterns',
    max_parallel: 2,
  } as const;

  const subtaskTwoMeta = {
    is_subtask: true,
    parent_task_id: parentTaskId,
    subtask_index: 1,
    total_subtasks: 2,
    subtask_preview: 'Inspect tool output rendering implementation',
    max_parallel: 2,
  } as const;

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
      delay: 3300,
      event: {
        event_type: 'iteration_start',
        agent_level: 'subagent',
        iteration: 1,
        total_iters: 1,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3350,
      event: {
        event_type: 'thinking',
        agent_level: 'subagent',
        iteration: 1,
        message_count: 1,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3400,
      event: {
        event_type: 'tool_call_start',
        agent_level: 'subagent',
        iteration: 1,
        call_id: 'mock-subagent-call-1',
        tool_name: 'web_search',
        arguments: {
          query: 'subagent console ux parallel timeline inspiration',
        },
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3500,
      event: {
        event_type: 'tool_call_stream',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-1',
        chunk: 'Summarizing multi-panel timelines from recent product launches...\n',
        is_complete: false,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3600,
      event: {
        event_type: 'tool_call_stream',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-1',
        chunk: 'Highlighted research from Cursor, Windsurf, and Devina.\n',
        is_complete: true,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3750,
      event: {
        event_type: 'tool_call_complete',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-1',
        tool_name: 'web_search',
        result:
          'Compiled documentation links covering responsive layouts and console UX patterns used in modern AI assistants.',
        duration: 780,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 3950,
      event: {
        event_type: 'task_complete',
        agent_level: 'subagent',
        final_answer:
          'Validated layout guidance from industry references and highlighted critical interaction affordances to emulate.',
        total_iterations: 1,
        total_tokens: 256,
        stop_reason: 'completed',
        duration: 900,
        ...subtaskOneMeta,
      },
    },
    {
      delay: 4050,
      event: {
        event_type: 'iteration_start',
        agent_level: 'subagent',
        iteration: 1,
        total_iters: 1,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4100,
      event: {
        event_type: 'thinking',
        agent_level: 'subagent',
        iteration: 1,
        message_count: 1,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4150,
      event: {
        event_type: 'tool_call_start',
        agent_level: 'subagent',
        iteration: 1,
        call_id: 'mock-subagent-call-2',
        tool_name: 'code_search',
        arguments: {
          path: 'web/components/agent/EventLine',
          query: 'subagent',
        },
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4325,
      event: {
        event_type: 'tool_call_stream',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-2',
        chunk: 'Traced ToolOutputCard props to ensure subtask metadata is surfaced...\n',
        is_complete: false,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4480,
      event: {
        event_type: 'tool_call_stream',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-2',
        chunk: 'Confirmed CSS tokens apply to subagent badges and dividers.\n',
        is_complete: true,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4650,
      event: {
        event_type: 'tool_call_complete',
        agent_level: 'subagent',
        call_id: 'mock-subagent-call-2',
        tool_name: 'code_search',
        result:
          'Identified relevant components handling tool output rendering and confirmed subagent styling tokens are applied.',
        duration: 640,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4850,
      event: {
        event_type: 'task_complete',
        agent_level: 'subagent',
        final_answer:
          'Confirmed ToolOutputCard handles metadata for subagent streams and recommended expanding automated coverage.',
        total_iterations: 1,
        total_tokens: 198,
        stop_reason: 'completed',
        duration: 880,
        ...subtaskTwoMeta,
      },
    },
    {
      delay: 4250,
      event: {
        event_type: 'think_complete',
        agent_level: 'core',
        iteration: 1,
        content: 'Ready to summarize the refactor recommendations.',
        tool_call_count: 1,
      },
    },
    {
      delay: 4325,
      event: {
        event_type: 'assistant_message',
        agent_level: 'core',
        iteration: 1,
        delta: 'Here are the key findings from the console audit:\n',
        final: false,
      },
    },
    {
      delay: 4400,
      event: {
        event_type: 'assistant_message',
        agent_level: 'core',
        iteration: 1,
        delta: '- Keep the input dock always visible so tasks are effortless.\n',
        final: false,
      },
    },
    {
      delay: 4480,
      event: {
        event_type: 'assistant_message',
        agent_level: 'core',
        iteration: 1,
        delta:
          '- Stream agent output in a dedicated column for clarity.\n- Provide reconnection affordances with the new console styling.',
        final: true,
      },
    },
    {
      delay: 4550,
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
