// ConnectionBanner - displays connection status and error messages
// Extracted from ConversationEventStream.tsx for better component separation

import { AlertCircle, Loader2, WifiOff } from 'lucide-react';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';

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
    <Card className="mx-auto flex h-full max-w-md flex-col items-center justify-center text-center">
      <CardContent className="flex flex-col items-center justify-center gap-4 px-6 py-6">
        <div className="flex items-center gap-3 text-sm font-semibold text-foreground">
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
          <Badge
            variant="destructive"
            className="flex items-center gap-2 px-3 py-2 text-[11px] font-medium"
          >
            <AlertCircle className="h-4 w-4" />
            <span>{error}</span>
          </Badge>
        )}

        <Button onClick={onReconnect} className="text-xs font-semibold">
          Retry
        </Button>
      </CardContent>
    </Card>
  );
}
