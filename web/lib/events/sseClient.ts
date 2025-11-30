import { apiClient } from '@/lib/api';
import { LegacyEventType, WorkflowEventType } from '@/lib/types';
import { EventPipeline } from './eventPipeline';

export interface SSEClientOptions {
  eventTypes: Array<WorkflowEventType | LegacyEventType | 'connected' | 'user_task'>;
  onOpen?: () => void;
  onError?: (error: Event | Error) => void;
  onClose?: () => void;
  reconnect?: boolean;
}

const WORKFLOW_EVENTS: WorkflowEventType[] = [
  'workflow.lifecycle.updated',
  'workflow.plan.generated',
  'workflow.node.started',
  'workflow.node.completed',
  'workflow.node.failed',
  'workflow.node.output.delta',
  'workflow.node.output.summary',
  'workflow.tool.started',
  'workflow.tool.progress',
  'workflow.tool.completed',
  'workflow.subflow.progress',
  'workflow.subflow.completed',
  'workflow.result.final',
  'workflow.result.cancelled',
  'workflow.diagnostic.context_compression',
  'workflow.diagnostic.tool_filtering',
  'workflow.diagnostic.browser_info',
  'workflow.diagnostic.environment_snapshot',
  'workflow.diagnostic.sandbox_progress',
  'workflow.diagnostic.context_snapshot',
];

const LEGACY_EVENTS: LegacyEventType[] = [
  'workflow_event',
  'iteration_start',
  'thinking',
  'assistant_message',
  'think_complete',
  'tool_call_start',
  'tool_call_stream',
  'tool_call_complete',
  'iteration_complete',
  'subagent_progress',
  'subagent_complete',
  'task_cancelled',
  'task_complete',
  'error',
  'research_plan',
  'step_started',
  'step_completed',
  'browser_info',
  'environment_snapshot',
  'sandbox_progress',
  'context_compression',
  'tool_filtering',
  'context_snapshot',
];

const DEFAULT_EVENTS: Array<WorkflowEventType | LegacyEventType | 'connected' | 'user_task'> =
  Array.from(new Set(['connected', 'user_task', ...WORKFLOW_EVENTS, ...LEGACY_EVENTS]));

export class SSEClient {
  private sessionId: string;
  private options: SSEClientOptions;
  private pipeline: EventPipeline;
  private eventSource: EventSource | null = null;
  private isDisposed: boolean = false;

  constructor(
    sessionId: string,
    pipeline: EventPipeline,
    options: Partial<SSEClientOptions> = {},
  ) {
    this.sessionId = sessionId;
    this.pipeline = pipeline;
    this.options = {
      eventTypes: options.eventTypes ?? DEFAULT_EVENTS,
      onOpen: options.onOpen,
      onError: options.onError,
      onClose: options.onClose,
      reconnect: options.reconnect ?? true,
    };
  }

  connect(accessToken?: string) {
    this.dispose();
    this.isDisposed = false;
    this.eventSource = apiClient.createSSEConnection(
      this.sessionId,
      accessToken,
    );
    const eventSource = this.eventSource;

    eventSource.onopen = () => {
      if (this.isDisposed) return;
      this.options.onOpen?.();
    };

    eventSource.onerror = (error) => {
      if (this.isDisposed) return;

      // Always surface errors so callers can decide whether to reconnect.
      // Some environments do not update readyState reliably when upstream
      // proxies terminate the stream, which would previously suppress the
      // callback and leave the UI stuck waiting for events.
      this.options.onError?.(error);
    };

    this.options.eventTypes.forEach((type) => {
      eventSource.addEventListener(type, (rawEvent: MessageEvent) => {
        if (this.isDisposed) return;

        try {
          const payload = JSON.parse(rawEvent.data);
          this.pipeline.process(payload);
        } catch (error) {
          console.error("[SSE] Failed to parse event payload", error);
        }
      });
    });
  }

  dispose() {
    this.isDisposed = true;
    if (this.eventSource) {
      this.eventSource.close();
      this.options.onClose?.();
      this.eventSource = null;
    }
  }
}
