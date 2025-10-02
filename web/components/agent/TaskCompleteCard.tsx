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
    <Card className="border-l-4 border-green-500 bg-gradient-to-r from-green-50 to-transparent">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-green-100 rounded-lg">
            <CheckCircle2 className="h-6 w-6 text-green-600" />
          </div>
          <div>
            <h3 className="font-semibold text-lg text-green-900">
              Task Completed
            </h3>
            <div className="flex items-center gap-3 text-sm text-green-700 mt-1">
              <span>{event.total_iterations} iterations</span>
              <span>•</span>
              <span>{event.total_tokens} tokens</span>
              <span>•</span>
              <span>{formatDuration(event.duration)}</span>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="bg-white p-4 rounded-md border border-green-100">
          <p className="text-sm font-medium text-gray-700 mb-2">Final Answer:</p>
          <div className="prose prose-sm max-w-none">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {event.final_answer}
            </ReactMarkdown>
          </div>
        </div>
        <div className="mt-3 text-xs text-gray-500">
          Stop reason: {event.stop_reason}
        </div>
      </CardContent>
    </Card>
  );
}
