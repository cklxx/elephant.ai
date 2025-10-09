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
    <div className="flex flex-col items-center justify-center h-full space-y-3">
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        {isReconnecting ? (
          <>
            <Loader2 className="h-4 w-4 animate-spin" />
            <span>
              Reconnecting…
              {typeof reconnectAttempts === 'number' && reconnectAttempts > 0 && (
                <span className="ml-1 text-muted-foreground/80">
                  (attempt {reconnectAttempts})
                </span>
              )}
            </span>
          </>
        ) : (
          <>
            <WifiOff className="h-4 w-4" />
            <span>Offline</span>
          </>
        )}
      </div>

      {error && (
        <div className="flex items-center gap-2 text-xs text-destructive">
          <AlertCircle className="h-3 w-3" />
          <span>{error}</span>
        </div>
      )}

      <button
        onClick={onReconnect}
        className="text-xs px-3 py-1.5 bg-primary text-primary-foreground rounded hover:opacity-90 transition-opacity"
      >
        Retry
      </button>
    </div>
  );
}
