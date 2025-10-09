'use client';

import { Badge } from '@/components/ui/badge';
import { WifiOff, RefreshCw } from 'lucide-react';
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
  error,
  onReconnect,
}: ConnectionStatusProps) {
  const t = useTranslation();

  if (connected && !reconnecting) {
    return null;
  }

  if (reconnecting) {
    return (
      <Badge variant="warning" className="flex items-center gap-1.5 animate-pulse text-[11px] font-semibold">
        <RefreshCw className="h-3 w-3 animate-spin" />
        <span>{t('connection.reconnecting')}</span>
      </Badge>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-2 animate-fadeIn">
      <Badge variant="error" className="flex items-center gap-1.5 text-[11px] font-semibold">
        <WifiOff className="h-3 w-3 animate-pulse" />
        <span>{t('connection.disconnected')}</span>
      </Badge>
      {error && (
        <span className="console-microcopy w-full break-words rounded border border-destructive/20 bg-destructive/5 px-2 py-1 text-destructive sm:w-auto">
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
