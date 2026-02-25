import { UseSSEOptions, UseSSEReturn, useSSE } from './useSSE';
import { useMockAgentStream } from './useMockAgentStream';

interface AgentEventStreamOptions extends UseSSEOptions {
  useMock?: boolean;
}

export function useAgentEventStream(
  sessionId: string | null,
  options: AgentEventStreamOptions = {}
): UseSSEReturn {
  const { useMock = false, ...rest } = options;

  const enabled = rest.enabled ?? true;

  const mockStream = useMockAgentStream(sessionId, {
    ...rest,
    enabled: enabled && useMock,
  });
  const sseStream = useSSE(sessionId, {
    ...rest,
    enabled: enabled && !useMock,
  });

  return useMock ? mockStream : sseStream;
}
