'use client';

import { WifiOff, RefreshCw } from 'lucide-react';
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
      <span className="console-quiet-chip animate-pulse gap-2 bg-muted/60 text-xs uppercase">
        <RefreshCw className="h-4 w-4 animate-spin" />
        <span>{t('connection.reconnecting')}</span>
      </span>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-3 animate-fadeIn">
      <span className="console-quiet-chip bg-destructive/15 text-destructive">
        <WifiOff className="h-4 w-4 animate-pulse" />
        <span>{t('connection.disconnected')}</span>
      </span>
      {error && (
        <span className="console-card bg-destructive/10 border-destructive/30 px-3 py-1 text-xs font-semibold uppercase tracking-[0.22em] text-destructive shadow-none">
          {error}
        </span>
      )}
      {onReconnect && (
        <button
          type="button"
          onClick={onReconnect}
          className="console-button console-button-primary text-xs uppercase"
        >
          {t('connection.reconnect')}
        </button>
      )}
    </div>
  );
}
