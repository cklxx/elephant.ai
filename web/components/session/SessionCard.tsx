'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Session } from '@/lib/types';
import { formatRelativeTime, truncate } from '@/lib/utils';
import { getLanguageLocale, useI18n } from '@/lib/i18n';
import { Trash2, GitBranch, ExternalLink } from 'lucide-react';
import Link from 'next/link';

interface SessionCardProps {
  session: Session;
  onDelete?: (sessionId: string) => void;
  onFork?: (sessionId: string) => void;
}

export function SessionCard({ session, onDelete, onFork }: SessionCardProps) {
  const { t, language } = useI18n();
  const locale = getLanguageLocale(language);
  const sessionTitle =
    typeof session.title === 'string' && session.title.trim()
      ? session.title.trim()
      : null;

  return (
    <Card className="transition hover:bg-white/10 backdrop-blur">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <CardTitle className="text-lg">
              {sessionTitle || t('sessions.card.title', { id: session.id.slice(0, 8) })}
            </CardTitle>
            <p className="text-sm text-muted-foreground mt-1">
              {formatRelativeTime(session.created_at, locale)}
            </p>
            {sessionTitle && (
              <p className="mt-1 text-[10px] font-mono text-muted-foreground/70">
                …{session.id.slice(-4)}
              </p>
            )}
          </div>
          <Link
            href={`/sessions/details?id=${encodeURIComponent(session.id)}`}
            aria-label={t('sessions.card.open')}
          >
            <Button size="sm" variant="ghost" aria-hidden="true">
              <ExternalLink className="h-4 w-4" />
            </Button>
          </Link>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {session.last_task && (
            <div>
              <p className="text-xs font-medium text-muted-foreground mb-1">
                {t('sessions.card.lastTask')}
              </p>
              <p className="text-sm text-foreground">
                {truncate(session.last_task, 100)}
              </p>
            </div>
          )}
          <div className="flex items-center justify-between text-xs text-muted-foreground">
            <span>{t('sessions.card.taskCount', { count: session.task_count })}</span>
            <span>
              {t('sessions.card.updated', {
                time: formatRelativeTime(session.updated_at, locale),
              })}
            </span>
          </div>
          <div className="flex items-center gap-2 pt-2">
            {onFork && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => onFork(session.id)}
                className="flex-1"
              >
                <GitBranch className="h-3 w-3 mr-1" />
                {t('sessions.card.fork')}
              </Button>
            )}
            {onDelete && (
              <Button
                size="sm"
                variant="destructive"
                onClick={() => onDelete(session.id)}
                aria-label={t('sessions.card.delete')}
              >
                <Trash2 className="h-3 w-3" />
              </Button>
            )}
          </div>
        </div>
      </CardContent>
    </Card>
  );
}
