// ConnectionBanner - displays connection status and error messages
// Extracted from TerminalOutput.tsx for better component separation

import { AlertCircle, Loader2, WifiOff } from 'lucide-react';

interface ConnectionBannerProps {
  isConnected: boolean;
  isReconnecting: boolean;
  error: string | null;
  reconnectAttempts?: number;
  onReconnect: () => void;
}

/**
 * ConnectionBanner - Connection status indicator
 * Displays reconnection status, errors, and reconnect button
 */
export function ConnectionBanner({
  isConnected,
  isReconnecting,
  error,
  reconnectAttempts,
  onReconnect,
}: ConnectionBannerProps) {
  // Only show banner when disconnected or has error
  if (isConnected && !error) {
    return null;
  }

  return (
    <div className="console-card mx-auto flex h-full max-w-md flex-col items-center justify-center gap-4 px-6 py-6 text-center">
      <div className="flex items-center gap-3 text-sm font-semibold uppercase tracking-[0.24em] text-foreground">
        {isReconnecting ? (
          <>
            <Loader2 className="h-5 w-5 animate-spin" />
            <span>
              Reconnecting
              {typeof reconnectAttempts === 'number' && reconnectAttempts > 0 && (
                <span className="ml-1 text-xs font-medium text-muted-foreground">
                  ({reconnectAttempts})
                </span>
              )}
            </span>
          </>
        ) : (
          <>
            <WifiOff className="h-5 w-5" />
            <span>Offline</span>
          </>
        )}
      </div>

      {error && (
        <div className="console-card bg-destructive/10 border-destructive/30 px-4 py-2 text-xs font-semibold uppercase tracking-[0.22em] text-destructive shadow-none">
          <div className="flex items-center justify-center gap-2">
            <AlertCircle className="h-4 w-4" />
            <span>{error}</span>
          </div>
        </div>
      )}

      <button
        onClick={onReconnect}
        className="console-button console-button-primary text-xs uppercase"
      >
        Retry
      </button>
    </div>
  );
}
