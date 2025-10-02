'use client';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ToolCallStartEvent, ToolCallCompleteEvent } from '@/lib/types';
import { getToolIcon, getToolColor, formatDuration, formatJSON } from '@/lib/utils';
import { CheckCircle2, XCircle, Loader2, ChevronDown, ChevronUp } from 'lucide-react';
import { useState } from 'react';
import { cn } from '@/lib/utils';

interface ToolCallCardProps {
  event: ToolCallStartEvent | ToolCallCompleteEvent;
  status: 'running' | 'complete' | 'error';
}

export function ToolCallCard({ event, status }: ToolCallCardProps) {
  const [expanded, setExpanded] = useState(false);
  const toolIcon = getToolIcon(event.tool_name);
  const toolColor = getToolColor(event.tool_name);

  const isComplete = event.event_type === 'tool_call_complete';
  const completeEvent = isComplete ? (event as ToolCallCompleteEvent) : null;
  const startEvent = event.event_type === 'tool_call_start' ? (event as ToolCallStartEvent) : null;

  return (
    <Card className={cn('border-l-4', toolColor)}>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <span className="text-3xl">{toolIcon}</span>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-lg">{event.tool_name}</h3>
                {status === 'running' && (
                  <Badge variant="info" className="flex items-center gap-1">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Running
                  </Badge>
                )}
                {status === 'complete' && !completeEvent?.error && (
                  <Badge variant="success" className="flex items-center gap-1">
                    <CheckCircle2 className="h-3 w-3" />
                    Completed
                  </Badge>
                )}
                {(status === 'error' || completeEvent?.error) && (
                  <Badge variant="error" className="flex items-center gap-1">
                    <XCircle className="h-3 w-3" />
                    Failed
                  </Badge>
                )}
              </div>
              <p className="text-sm text-gray-500 mt-1">
                {isComplete && completeEvent?.duration
                  ? `Completed in ${formatDuration(completeEvent.duration)}`
                  : 'Executing...'}
              </p>
            </div>
          </div>
          <button
            onClick={() => setExpanded(!expanded)}
            className="text-gray-500 hover:text-gray-700 transition-colors"
          >
            {expanded ? (
              <ChevronUp className="h-5 w-5" />
            ) : (
              <ChevronDown className="h-5 w-5" />
            )}
          </button>
        </div>
      </CardHeader>

      {expanded && (
        <CardContent className="space-y-4">
          {/* Arguments */}
          {(startEvent?.arguments || (event as any).arguments) && (
            <div>
              <p className="text-sm font-medium text-gray-700 mb-2">Arguments:</p>
              <pre className="bg-gray-50 p-3 rounded-md text-xs overflow-x-auto border border-gray-200">
                {formatJSON((startEvent?.arguments || (event as any).arguments))}
              </pre>
            </div>
          )}

          {/* Result */}
          {completeEvent?.result && (
            <div>
              <p className="text-sm font-medium text-gray-700 mb-2">Result:</p>
              <pre className="bg-gray-50 p-3 rounded-md text-xs overflow-x-auto border border-gray-200 max-h-96">
                {completeEvent.result}
              </pre>
            </div>
          )}

          {/* Error */}
          {completeEvent?.error && (
            <div>
              <p className="text-sm font-medium text-red-700 mb-2">Error:</p>
              <pre className="bg-red-50 p-3 rounded-md text-xs overflow-x-auto border border-red-200">
                {completeEvent.error}
              </pre>
            </div>
          )}

          {/* Call ID */}
          <div className="text-xs text-gray-400">
            Call ID: {event.call_id}
          </div>
        </CardContent>
      )}
    </Card>
  );
}
