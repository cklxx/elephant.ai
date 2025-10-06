// Integration hook that bridges useSSE with useAgentStreamStore
// This allows existing components to continue working while benefiting from the new store

import { useSSE } from './useSSE';
import { useAgentStreamStore } from './useAgentStreamStore';

/**
 * Hook that integrates SSE events with the Zustand store
 * Use this instead of useSSE directly when you want store-based event management
 */
export function useAgentStreamIntegration(sessionId: string | null) {
  const {
    events,
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    clearEvents: clearSSEEvents,
    reconnect,
  } = useSSE(sessionId, {
    // Callback to sync events to store in real-time
    // Each event is added individually as it arrives from SSE
    onEvent: (event) => {
      useAgentStreamStore.getState().addEvent(event);
    },
  });

  const clearStoreEvents = useAgentStreamStore((state) => state.clearEvents);

  // NOTE: We do NOT sync the events array to store in useEffect
  // because each event is already added via onEvent callback above.
  // Syncing the entire array would cause duplicates.

  // Unified clear function that clears both SSE and store
  const clearEvents = () => {
    clearSSEEvents();
    clearStoreEvents();
  };

  return {
    // SSE connection state
    isConnected,
    isReconnecting,
    error,
    reconnectAttempts,
    reconnect,
    clearEvents,

    // Store state (via store selectors)
    // Components should use store selectors directly for better performance
    // This is just for backwards compatibility
    events,
  };
}
