'use client';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ErrorEvent } from '@/lib/types';
import { AlertCircle } from 'lucide-react';

interface ErrorCardProps {
  event: ErrorEvent;
}

export function ErrorCard({ event }: ErrorCardProps) {
  return (
    <Card className="console-card border-l-4 border-destructive animate-fadeIn overflow-hidden">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-3 bg-destructive rounded-md animate-pulse">
              <AlertCircle className="h-6 w-6 text-destructive-foreground" />
            </div>
            <div>
              <h3 className="console-heading text-lg text-destructive">Error</h3>
              <p className="console-caption">
                Phase: {event.phase} (Iteration {event.iteration})
              </p>
            </div>
          </div>
          {event.recoverable && (
            <Badge variant="warning" className="animate-fadeIn">Recoverable</Badge>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="console-card p-4 bg-destructive/5">
          <pre className="text-sm text-destructive whitespace-pre-wrap font-mono leading-relaxed">
            {event.error}
          </pre>
        </div>
      </CardContent>
    </Card>
  );
}
