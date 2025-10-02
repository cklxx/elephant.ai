'use client';

import { Badge } from '@/components/ui/badge';
import { Wifi, WifiOff, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';

interface ConnectionStatusProps {
  connected: boolean;
  reconnecting: boolean;
  reconnectAttempts?: number;
  error?: string | null;
  onReconnect?: () => void;
}

export function ConnectionStatus({
  connected,
  reconnecting,
  reconnectAttempts = 0,
  error,
  onReconnect,
}: ConnectionStatusProps) {
  if (connected) {
    return (
      <Badge variant="success" className="flex items-center gap-1.5">
        <Wifi className="h-3 w-3" />
        Connected
      </Badge>
    );
  }

  if (reconnecting) {
    return (
      <Badge variant="warning" className="flex items-center gap-1.5">
        <RefreshCw className="h-3 w-3 animate-spin" />
        Reconnecting... (Attempt {reconnectAttempts})
      </Badge>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <Badge variant="error" className="flex items-center gap-1.5">
        <WifiOff className="h-3 w-3" />
        Disconnected
      </Badge>
      {error && (
        <span className="text-xs text-red-600">{error}</span>
      )}
      {onReconnect && (
        <Button
          size="sm"
          variant="outline"
          onClick={onReconnect}
          className="h-6 px-2 text-xs"
        >
          Reconnect
        </Button>
      )}
    </div>
  );
}
