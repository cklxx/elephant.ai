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
    <Card className="manus-card border-l-4 border-green-600 animate-fadeIn overflow-hidden">
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <div className="p-3 bg-green-600 rounded-md animate-fadeIn">
            <CheckCircle2 className="h-6 w-6 text-white" />
          </div>
          <div>
            <h3 className="manus-heading text-xl text-green-700">
              Task Completed
            </h3>
            <div className="flex items-center gap-3 text-sm manus-caption mt-1">
              <span className="flex items-center gap-1">
                <span className="w-1.5 h-1.5 bg-green-600 rounded-full"></span>
                {event.total_iterations} iterations
              </span>
              <span>•</span>
              <span className="flex items-center gap-1">
                <span className="w-1.5 h-1.5 bg-green-600 rounded-full"></span>
                {event.total_tokens} tokens
              </span>
              <span>•</span>
              <span className="flex items-center gap-1">
                <span className="w-1.5 h-1.5 bg-green-600 rounded-full"></span>
                {formatDuration(event.duration)}
              </span>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <div className="manus-card p-5 bg-green-50/50">
          <p className="manus-subheading text-sm mb-3 flex items-center gap-2">
            <span className="w-1.5 h-1.5 bg-green-600 rounded-full"></span>
            Final Answer:
          </p>
          <div className="prose prose-sm max-w-none">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {event.final_answer}
            </ReactMarkdown>
          </div>
        </div>
        <div className="mt-4 flex items-center gap-2 text-xs manus-caption font-mono">
          <span className="manus-badge-outline">
            Stop reason: {event.stop_reason}
          </span>
        </div>
      </CardContent>
    </Card>
  );
}
