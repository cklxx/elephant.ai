"use client";

import { WifiOff, RefreshCw } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';
import { Badge } from '@/components/ui/badge';
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
  error,
  onReconnect,
}: ConnectionStatusProps) {
  const t = useTranslation();

  if (connected && !reconnecting) {
    return null;
  }

  if (reconnecting) {
    return (
      <Badge variant="outline" className="gap-2 text-[11px] uppercase tracking-[0.2em]">
        <RefreshCw className="h-4 w-4 animate-spin" />
        <span>{t('connection.reconnecting')}</span>
      </Badge>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-3">
      <Badge variant="destructive" className="gap-2">
        <WifiOff className="h-4 w-4 animate-pulse" />
        <span>{t('connection.disconnected')}</span>
      </Badge>
      {error && (
        <Badge variant="destructive" className="px-3 py-1 text-[11px] uppercase tracking-[0.22em]">
          {error}
        </Badge>
      )}
      {onReconnect && (
        <Button type="button" size="sm" onClick={onReconnect} className="text-[11px] uppercase">
          {t('connection.reconnect')}
        </Button>
      )}
    </div>
  );
}
