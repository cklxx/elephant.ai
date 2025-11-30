import { AnyAgentEvent, WorkflowInputReceivedEvent, eventMatches } from '@/lib/types';

const STREAM_FLAG_TRUE = '1';
const STREAM_FLAG_FALSE = '0';

/**
 * Builds a stable signature for deduping logically identical events across the
 * legacy and workflow namespaces. The signature is intentionally narrow: it
 * focuses on identifiers and streamed content so that streaming updates with
 * new chunks still flow through while verbatim replays and dual-emitted pairs
 * are collapsed.
 */
export function buildEventSignature(event: AnyAgentEvent): string {
  if (event.event_type === 'workflow.input.received') {
    const taskEvent = event as WorkflowInputReceivedEvent;
    return [
      taskEvent.event_type,
      taskEvent.session_id ?? '',
      taskEvent.task_id ?? '',
      taskEvent.task,
    ].join('|');
  }

  const baseParts = [
    event.event_type,
    event.timestamp ?? '',
    event.session_id ?? '',
    'task_id' in event && event.task_id ? event.task_id : '',
  ];

  if ('call_id' in event && event.call_id) {
    baseParts.push(event.call_id);
  }
  if ('iteration' in event && typeof event.iteration === 'number') {
    baseParts.push(String(event.iteration));
  }
  if ('chunk' in event && typeof event.chunk === 'string') {
    baseParts.push(event.chunk);
  }
  if ('delta' in event && typeof event.delta === 'string') {
    baseParts.push(event.delta);
  }
  if ('result' in event && typeof event.result === 'string') {
    baseParts.push(event.result);
  }
  if ('error' in event && typeof event.error === 'string') {
    baseParts.push(event.error);
  }
  if ('final_answer' in event && typeof event.final_answer === 'string') {
    baseParts.push(event.final_answer);
  }
  if ('task' in event && typeof event.task === 'string') {
    baseParts.push(event.task);
  }
  if ('created_at' in event) {
    const createdAt = (event as { created_at?: unknown }).created_at;
    if (typeof createdAt === 'string') {
      baseParts.push(createdAt);
    }
  }

  if (eventMatches(event, 'workflow.result.final')) {
    const isStreaming =
      'is_streaming' in event && typeof event.is_streaming === 'boolean'
        ? event.is_streaming
        : undefined;
    const streamFinished =
      'stream_finished' in event && typeof event.stream_finished === 'boolean'
        ? event.stream_finished
        : undefined;

    if (isStreaming !== undefined) {
      baseParts.push(isStreaming ? STREAM_FLAG_TRUE : STREAM_FLAG_FALSE);
    }
    if (streamFinished !== undefined) {
      baseParts.push(streamFinished ? STREAM_FLAG_TRUE : STREAM_FLAG_FALSE);
    }
  }

  return baseParts.join('|');
}
