'use client';

import { Badge } from '@/components/ui/badge';
import { Wifi, WifiOff, RefreshCw } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { useTranslation } from '@/lib/i18n';

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
  const t = useTranslation();

  if (connected) {
    return (
      <Badge variant="success" className="flex items-center gap-2 animate-fadeIn">
        <div className="relative flex items-center">
          <span className="absolute w-3 h-3 bg-green-400 rounded-full animate-ping opacity-75"></span>
          <Wifi className="relative h-3 w-3" />
        </div>
        <span className="font-semibold">{t('connection.connected')}</span>
      </Badge>
    );
  }

  if (reconnecting) {
    return (
      <Badge variant="warning" className="flex items-center gap-2 animate-pulse">
        <RefreshCw className="h-3.5 w-3.5 animate-spin" />
        <span className="font-semibold">
          {t('connection.reconnecting', { attempt: reconnectAttempts })}
        </span>
      </Badge>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-2 animate-fadeIn">
      <Badge variant="error" className="flex items-center gap-2 shrink-0">
        <WifiOff className="h-3 w-3 animate-pulse" />
        <span className="font-semibold">{t('connection.disconnected')}</span>
      </Badge>
      {error && (
        <span className="w-full break-words rounded border border-red-200 bg-red-50 px-2 py-1 text-xs font-medium text-red-600 sm:inline-flex sm:w-auto sm:items-center">
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
          {t('connection.reconnect')}
        </Button>
      )}
    </div>
  );
}
