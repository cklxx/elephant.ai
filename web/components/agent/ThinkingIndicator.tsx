'use client';

import { Card } from '@/components/ui/card';
import { Brain, Loader2 } from 'lucide-react';

export function ThinkingIndicator() {
  return (
    <Card className="border-l-4 border-gray-400 bg-gradient-to-r from-gray-50/80 to-transparent backdrop-blur-sm shadow-soft animate-slideIn overflow-hidden">
      <div className="absolute top-0 right-0 w-24 h-24 bg-gray-200/30 rounded-full blur-2xl animate-pulse-soft"></div>
      <div className="p-4 flex items-center gap-3 relative">
        <div className="p-3 bg-gradient-to-br from-gray-400 to-gray-500 rounded-xl shadow-md animate-pulse-soft">
          <Brain className="h-5 w-5 text-white" />
        </div>
        <div className="flex items-center gap-3">
          <Loader2 className="h-5 w-5 animate-spin text-gray-600" />
          <span className="text-sm font-semibold text-gray-700">
            Agent is thinking...
          </span>
        </div>
        <div className="ml-auto flex gap-1">
          <span className="w-2 h-2 bg-gray-400 rounded-full animate-pulse" style={{ animationDelay: '0ms' }}></span>
          <span className="w-2 h-2 bg-gray-400 rounded-full animate-pulse" style={{ animationDelay: '150ms' }}></span>
          <span className="w-2 h-2 bg-gray-400 rounded-full animate-pulse" style={{ animationDelay: '300ms' }}></span>
        </div>
      </div>
    </Card>
  );
}
