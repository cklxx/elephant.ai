import { apiClient } from '@/lib/api';
import { WorkflowEventType } from '@/lib/types';
import { createLogger } from '@/lib/logger';
import { EventPipeline } from './eventPipeline';
import type { SSEReplayMode } from '@/lib/api';

const log = createLogger("SSE");

export interface SSEClientOptions {
  eventTypes: Array<WorkflowEventType | 'connected'>;
  replay?: SSEReplayMode;
  onOpen?: () => void;
  onError?: (error: Event | Error) => void;
  onClose?: () => void;
  reconnect?: boolean;
}

const WORKFLOW_EVENTS: WorkflowEventType[] = [
  'workflow.node.started',
  'workflow.node.completed',
  'workflow.node.failed',
  'workflow.node.output.delta',
  'workflow.node.output.summary',
  'workflow.tool.started',
  'workflow.tool.progress',
  'workflow.tool.completed',
  'workflow.artifact.manifest',
  'workflow.input.received',
  'workflow.subflow.progress',
  'workflow.subflow.completed',
  'workflow.result.final',
  'workflow.result.cancelled',
  'workflow.diagnostic.error',
  'workflow.diagnostic.context_compression',
  'workflow.diagnostic.tool_filtering',
  'workflow.diagnostic.environment_snapshot',
];

const DEFAULT_EVENTS: Array<WorkflowEventType | 'connected'> = Array.from(new Set(['connected', ...WORKFLOW_EVENTS]));

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
      replay: options.replay,
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
      { replay: this.options.replay },
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
          log.error("Failed to parse event payload", { error });
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
