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
    <Card className="border-l-4 border-red-500 bg-gradient-to-br from-red-50/80 via-white to-transparent backdrop-blur-sm shadow-medium hover-lift animate-slideIn overflow-hidden">
      <div className="absolute top-0 right-0 w-32 h-32 bg-red-100/30 rounded-full blur-3xl"></div>
      <CardHeader className="pb-3 relative">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-3 bg-gradient-to-br from-red-500 to-red-600 rounded-xl shadow-lg animate-pulse">
              <AlertCircle className="h-6 w-6 text-white" />
            </div>
            <div>
              <h3 className="font-semibold text-lg text-red-900">Error</h3>
              <p className="text-sm text-red-700 font-medium">
                Phase: {event.phase} (Iteration {event.iteration})
              </p>
            </div>
          </div>
          {event.recoverable && (
            <Badge variant="warning" className="animate-scaleIn">Recoverable</Badge>
          )}
        </div>
      </CardHeader>
      <CardContent className="relative">
        <div className="bg-white/60 backdrop-blur-sm p-4 rounded-xl border border-red-200/50 shadow-soft">
          <pre className="text-sm text-red-900 whitespace-pre-wrap font-mono leading-relaxed">
            {event.error}
          </pre>
        </div>
      </CardContent>
    </Card>
  );
}
