'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Session } from '@/lib/types';
import { formatRelativeTime, truncate } from '@/lib/utils';
import { Trash2, GitBranch, ExternalLink } from 'lucide-react';
import Link from 'next/link';

interface SessionCardProps {
  session: Session;
  onDelete?: (sessionId: string) => void;
  onFork?: (sessionId: string) => void;
}

export function SessionCard({ session, onDelete, onFork }: SessionCardProps) {
  return (
    <Card className="hover:shadow-lg transition-shadow">
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <CardTitle className="text-lg">
              Session {session.id.slice(0, 8)}
            </CardTitle>
            <p className="text-sm text-gray-500 mt-1">
              {formatRelativeTime(session.created_at)}
            </p>
          </div>
          <Link href={`/sessions/${session.id}`}>
            <Button size="sm" variant="ghost">
              <ExternalLink className="h-4 w-4" />
            </Button>
          </Link>
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {session.last_task && (
            <div>
              <p className="text-xs font-medium text-gray-600 mb-1">Last Task:</p>
              <p className="text-sm text-gray-900">
                {truncate(session.last_task, 100)}
              </p>
            </div>
          )}
          <div className="flex items-center justify-between text-xs text-gray-500">
            <span>{session.task_count} tasks</span>
            <span>Updated {formatRelativeTime(session.updated_at)}</span>
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
                Fork
              </Button>
            )}
            {onDelete && (
              <Button
                size="sm"
                variant="destructive"
                onClick={() => onDelete(session.id)}
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
