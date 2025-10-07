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

  if (useMock) {
    return useMockAgentStream(sessionId, rest);
  }

  return useSSE(sessionId, rest);
}
