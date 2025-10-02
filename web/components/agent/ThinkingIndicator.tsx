'use client';

import { Card } from '@/components/ui/card';
import { Brain, Loader2 } from 'lucide-react';

export function ThinkingIndicator() {
  return (
    <Card className="border-l-4 border-gray-300 bg-gray-50">
      <div className="p-4 flex items-center gap-3">
        <div className="p-2 bg-gray-200 rounded-lg">
          <Brain className="h-5 w-5 text-gray-600" />
        </div>
        <div className="flex items-center gap-2">
          <Loader2 className="h-4 w-4 animate-spin text-gray-600" />
          <span className="text-sm font-medium text-gray-700">
            Agent is thinking...
          </span>
        </div>
      </div>
    </Card>
  );
}
