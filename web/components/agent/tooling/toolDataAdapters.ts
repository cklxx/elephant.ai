import {
  WorkflowToolStartedEvent,
  WorkflowToolCompletedEvent,
  eventMatches,
} from '@/lib/types';
import { isWorkflowToolCompletedEvent, isWorkflowToolStartedEvent } from '@/lib/typeGuards';
import { RendererContext } from './toolRenderers';

export type ToolCallStatus = 'running' | 'done' | 'error';

export interface ToolCallAdapterInput {
  event: WorkflowToolStartedEvent | WorkflowToolCompletedEvent;
  pairedStart?: WorkflowToolStartedEvent;
  status: ToolCallStatus;
}

export interface ToolCallAdapterResult {
  callId: string;
  toolName: string;
  context: Omit<RendererContext, 'labels'>;
  durationMs?: number;
}

export function adaptToolCallForRenderer({
  event,
  pairedStart,
  status,
}: ToolCallAdapterInput): ToolCallAdapterResult {
  const startEvent = isWorkflowToolStartedEvent(event) ? event : pairedStart ?? null;
  const completeEvent = isWorkflowToolCompletedEvent(event) ? event : null;
  const toolName = completeEvent?.tool_name ?? startEvent?.tool_name ?? event.tool_name;
  const callId = completeEvent?.call_id ?? startEvent?.call_id ?? event.call_id;

  const streamContent =
    startEvent && typeof (startEvent as any).stream_content === 'string'
      ? ((startEvent as any).stream_content as string)
      : undefined;
  const streamTimestamp =
    startEvent && typeof (startEvent as any).last_stream_timestamp === 'string'
      ? ((startEvent as any).last_stream_timestamp as string)
      : undefined;

  return {
    callId,
    toolName,
    context: {
      startEvent,
      completeEvent,
      status,
      toolName,
      streamContent,
      streamTimestamp,
    },
    durationMs: completeEvent?.duration,
  };
}
