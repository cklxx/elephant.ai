'use client';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { TaskCompleteEvent } from '@/lib/types';
import { CheckCircle2 } from 'lucide-react';
import { formatDuration } from '@/lib/utils';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

interface TaskCompleteCardProps {
  event: TaskCompleteEvent;
}

export function TaskCompleteCard({ event }: TaskCompleteCardProps) {
  return (
    <Card className="border-l-4 border-green-500 bg-gradient-to-br from-green-50/80 via-white to-transparent backdrop-blur-sm shadow-strong hover-lift animate-scaleIn overflow-hidden">
      <div className="absolute top-0 right-0 w-40 h-40 bg-green-100/40 rounded-full blur-3xl"></div>
      <CardHeader className="pb-3 relative">
        <div className="flex items-center gap-3">
          <div className="p-3 bg-gradient-to-br from-green-500 to-green-600 rounded-xl shadow-lg hover-glow animate-scaleIn">
            <CheckCircle2 className="h-6 w-6 text-white" />
          </div>
          <div>
            <h3 className="font-semibold text-xl text-green-900">
              Task Completed
            </h3>
            <div className="flex items-center gap-3 text-sm text-green-700 mt-1 font-medium">
              <span className="flex items-center gap-1">
                <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
                {event.total_iterations} iterations
              </span>
              <span>•</span>
              <span className="flex items-center gap-1">
                <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
                {event.total_tokens} tokens
              </span>
              <span>•</span>
              <span className="flex items-center gap-1">
                <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
                {formatDuration(event.duration)}
              </span>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent className="relative">
        <div className="bg-white/70 backdrop-blur-sm p-5 rounded-xl border border-green-200/50 shadow-medium">
          <p className="text-sm font-semibold text-gray-700 mb-3 flex items-center gap-2">
            <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
            Final Answer:
          </p>
          <div className="prose prose-sm max-w-none">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {event.final_answer}
            </ReactMarkdown>
          </div>
        </div>
        <div className="mt-4 flex items-center gap-2 text-xs text-gray-500 font-mono">
          <span className="px-2 py-1 bg-gray-100 rounded border border-gray-200">
            Stop reason: {event.stop_reason}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
