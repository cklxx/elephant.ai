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
      <Badge variant="success" className="flex items-center gap-2 animate-fadeIn">
        <div className="relative flex items-center">
          <span className="absolute w-3 h-3 bg-green-400 rounded-full animate-ping opacity-75"></span>
          <Wifi className="h-3 w-3 relative" />
        </div>
        <span className="font-semibold">Connected</span>
      </Badge>
    );
  }

  if (reconnecting) {
    return (
      <Badge variant="warning" className="flex items-center gap-2 animate-pulse">
        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
        <span className="font-semibold">Reconnecting... (Attempt {reconnectAttempts})</span>
      </Badge>
    );
  }

  return (
    <div className="flex items-center gap-3 animate-fadeIn">
      <Badge variant="error" className="flex items-center gap-2">
        <WifiOff className="h-3 w-3 animate-pulse" />
        <span className="font-semibold">Disconnected</span>
      </Badge>
      {error && (
        <span className="text-xs text-red-600 font-medium bg-red-50 px-2 py-1 rounded border border-red-200">
          {error}
        </span>
      )}
      {onReconnect && (
        <Button
          size="sm"
          variant="outline"
          onClick={onReconnect}
          className="h-7 px-3 text-xs font-semibold hover-subtle"
        >
          Reconnect
        </Button>
      )}
    </div>
  );
}
