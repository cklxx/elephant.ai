import {
  AnyAgentEvent,
  SubagentCompleteEvent,
  SubagentProgressEvent,
} from '@/lib/types';

type DerivedEvent = SubagentProgressEvent | SubagentCompleteEvent;

interface ProgressState {
  total: number;
  completed: Set<number>;
  tokens: number;
  toolCalls: number;
  failures: number;
}

/**
 * SubagentEventDeriver produces synthetic subagent_progress and
 * subagent_complete events from raw subtask streams. The backend emits
 * subtask-wrapped task/tool events (is_subtask=true) but not the higher-level
 * aggregation events the UI expects. This helper fills that gap by tracking
 * per-parent task progress and emitting normalized events back into the
 * pipeline.
 */
export class SubagentEventDeriver {
  private stateByParent: Map<string, ProgressState> = new Map();

  derive(event: AnyAgentEvent): DerivedEvent[] {
    const isSubtaskFlag = 'is_subtask' in event && Boolean(event.is_subtask);
    const isSubagentLevel = event.agent_level === 'subagent';
    const hasParentTask = Boolean(event.parent_task_id) && event.parent_task_id !== event.task_id;
    const hasSubagentCallPrefix =
      'call_id' in event && typeof event.call_id === 'string' && event.call_id.startsWith('subagent:');

    // Multiple heuristics are used to detect delegated agent streams because
    // some payloads may omit is_subtask while still being tagged with
    // agent_level=subagent or the subagent tool call prefix.
    const isDelegated = isSubtaskFlag || isSubagentLevel || hasParentTask || hasSubagentCallPrefix;
    if (!isDelegated) {
      return [];
    }

    const subtaskIndex =
      'subtask_index' in event && typeof event.subtask_index === 'number' ? event.subtask_index : undefined;
    const parentTaskId =
      ('parent_task_id' in event && event.parent_task_id) || (isDelegated ? event.task_id : undefined);
    const totalSubtasks = 'total_subtasks' in event ? event.total_subtasks : undefined;

    if (!parentTaskId) {
      return [];
    }

    const key = `${event.session_id}:${parentTaskId}`;
    const existingState = this.stateByParent.get(key);
    const inferredTotal = typeof subtaskIndex === 'number' ? subtaskIndex + 1 : undefined;
    const candidateTotals = [totalSubtasks, existingState?.total, inferredTotal].filter(
      (value): value is number => typeof value === 'number',
    );
    const resolvedTotal = candidateTotals.length > 0 ? Math.max(...candidateTotals) : undefined;

    const state = existingState ?? {
      total: resolvedTotal ?? 0,
      completed: new Set<number>(),
      tokens: 0,
      toolCalls: 0,
      failures: 0,
    };

    // Persist the maximum observed total for consistency across events
    if (typeof resolvedTotal === 'number') {
      state.total = Math.max(state.total, resolvedTotal ?? state.total);
    }

    if (event.event_type.startsWith('tool_call')) {
      state.toolCalls += 1;
    }

    let tokensToAdd = 0;
    if ('total_tokens' in event && typeof event.total_tokens === 'number') {
      tokensToAdd = event.total_tokens;
    } else if ('tokens_used' in event && typeof event.tokens_used === 'number') {
      tokensToAdd = event.tokens_used;
    }
    state.tokens += tokensToAdd;

    const completionEvents = new Set([
      'task_complete',
      'task_cancelled',
      'error',
    ] as const);

    let markComplete = false;
    if (completionEvents.has(event.event_type as typeof completionEvents extends Set<infer T> ? T : never)) {
      const index =
        'subtask_index' in event && typeof event.subtask_index === 'number'
          ? event.subtask_index
          : state.completed.size;
      state.completed.add(index);
      markComplete = true;

      if (event.event_type !== 'task_complete') {
        state.failures += 1;
      }

      if (state.total === 0) {
        state.total = Math.max(state.completed.size, typeof inferredTotal === 'number' ? inferredTotal : 1);
      }
    }

    this.stateByParent.set(key, state);

    if (!markComplete && tokensToAdd === 0 && !event.event_type.startsWith('tool_call')) {
      return [];
    }

    const baseContext = {
      timestamp: event.timestamp ?? new Date().toISOString(),
      agent_level: isDelegated ? 'subagent' : event.agent_level ?? 'core',
      session_id: event.session_id ?? '',
      task_id: parentTaskId,
      parent_task_id: parentTaskId,
    } satisfies Pick<DerivedEvent, 'timestamp' | 'agent_level' | 'session_id' | 'task_id' | 'parent_task_id'>;

    const progressEvent: SubagentProgressEvent = {
      ...baseContext,
      event_type: 'subagent_progress',
      completed: state.completed.size,
      total: state.total,
      tokens: state.tokens,
      tool_calls: state.toolCalls,
    };

    const derived: DerivedEvent[] = [progressEvent];

    const shouldEmitCompletion =
      state.total > 0 && state.completed.size > 0 && state.completed.size >= state.total;

    if (shouldEmitCompletion) {
      const successCount = state.completed.size - state.failures;
      const completeEvent: SubagentCompleteEvent = {
        ...baseContext,
        event_type: 'subagent_complete',
        total: state.total,
        success: successCount,
        failed: state.failures,
        tokens: state.tokens,
        tool_calls: state.toolCalls,
      };
      derived.push(completeEvent);
    }

    return derived;
  }
}
