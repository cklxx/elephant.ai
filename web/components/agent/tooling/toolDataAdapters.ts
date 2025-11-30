import {
  ToolCallStartEvent,
  ToolCallCompleteEvent,
  eventMatches,
} from '@/lib/types';
import { isToolCallStartEvent } from '@/lib/typeGuards';
import { RendererContext } from './toolRenderers';

export type ToolCallStatus = 'running' | 'done' | 'error';

export interface ToolCallAdapterInput {
  event: ToolCallStartEvent | ToolCallCompleteEvent;
  pairedStart?: ToolCallStartEvent;
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
  const startEvent = isToolCallStartEvent(event) ? event : pairedStart ?? null;
  const completeEvent = eventMatches(event, 'workflow.tool.completed', 'tool_call_complete') ? event : null;
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
