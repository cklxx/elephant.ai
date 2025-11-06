import { apiClient } from '@/lib/api';
import { AnyAgentEvent } from '@/lib/types';
import { EventPipeline } from './eventPipeline';

export interface SSEClientOptions {
  eventTypes: AnyAgentEvent['event_type'][];
  onOpen?: () => void;
  onError?: (error: Event | Error) => void;
  onClose?: () => void;
  reconnect?: boolean;
}

const DEFAULT_EVENTS: AnyAgentEvent['event_type'][] = [
  'connected',
  'task_analysis',
  'iteration_start',
  'thinking',
  'think_complete',
  'tool_call_start',
  'tool_call_stream',
  'tool_call_complete',
  'iteration_complete',
  'task_complete',
  'error',
  'research_plan',
  'step_started',
  'step_completed',
  'browser_info',
  'environment_snapshot',
  'sandbox_progress',
];

export class SSEClient {
  private sessionId: string;
  private options: SSEClientOptions;
  private pipeline: EventPipeline;
  private eventSource: EventSource | null = null;
  private isDisposed: boolean = false;

  constructor(sessionId: string, pipeline: EventPipeline, options: Partial<SSEClientOptions> = {}) {
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

  connect() {
    this.dispose();
    this.isDisposed = false;
    this.eventSource = apiClient.createSSEConnection(this.sessionId);
    const eventSource = this.eventSource;

    eventSource.onopen = () => {
      if (this.isDisposed) return;
      this.options.onOpen?.();
    };

    eventSource.onerror = (error) => {
      if (this.isDisposed) return;

      // Only trigger error callback if connection is actually broken
      // EventSource.readyState: 0=CONNECTING, 1=OPEN, 2=CLOSED
      const closedState =
        (eventSource as unknown as { CLOSED?: number }).CLOSED ??
        (typeof EventSource !== 'undefined' ? EventSource.CLOSED : undefined);
      if (closedState === undefined || eventSource.readyState === closedState) {
        this.options.onError?.(error);
      }
    };

    this.options.eventTypes.forEach((type) => {
      eventSource.addEventListener(type, (rawEvent: MessageEvent) => {
        if (this.isDisposed) return;

        try {
          const payload = JSON.parse(rawEvent.data);
          this.pipeline.process(payload);
        } catch (error) {
          console.error('[SSE] Failed to parse event payload', error);
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
