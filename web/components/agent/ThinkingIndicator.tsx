'use client';

import { Card } from '@/components/ui/card';
import { Brain, Loader2 } from 'lucide-react';

export function ThinkingIndicator() {
  return (
    <Card className="manus-card border-l-4 border-muted-foreground animate-fadeIn overflow-hidden">
      <div className="p-4 flex items-center gap-3">
        <div className="p-3 bg-muted rounded-md animate-pulse">
          <Brain className="h-5 w-5 text-muted-foreground" />
        </div>
        <div className="flex items-center gap-3">
          <Loader2 className="h-5 w-5 animate-spin text-muted-foreground" />
          <span className="text-sm manus-subheading">
            Agent is thinking...
          </span>
        </div>
        <div className="ml-auto flex gap-1">
          <span className="w-2 h-2 bg-muted-foreground rounded-full animate-pulse" style={{ animationDelay: '0ms' }}></span>
          <span className="w-2 h-2 bg-muted-foreground rounded-full animate-pulse" style={{ animationDelay: '150ms' }}></span>
          <span className="w-2 h-2 bg-muted-foreground rounded-full animate-pulse" style={{ animationDelay: '300ms' }}></span>
        </div>
      </div>
    </Card>
  );
}
