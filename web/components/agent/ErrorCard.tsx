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
    <Card className="border-l-4 border-red-500 bg-gradient-to-r from-red-50 to-transparent">
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 bg-red-100 rounded-lg">
              <AlertCircle className="h-6 w-6 text-red-600" />
            </div>
            <div>
              <h3 className="font-semibold text-lg text-red-900">Error</h3>
              <p className="text-sm text-red-700">
                Phase: {event.phase} (Iteration {event.iteration})
              </p>
            </div>
          </div>
          {event.recoverable && (
            <Badge variant="warning">Recoverable</Badge>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="bg-white p-4 rounded-md border border-red-100">
          <pre className="text-sm text-red-900 whitespace-pre-wrap font-mono">
            {event.error}
          </pre>
        </div>
      </CardContent>
    </Card>
  );
}
