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
    <Card className={cn('border-l-4 shadow-medium hover-lift animate-slideIn overflow-hidden bg-white/80 backdrop-blur-sm', toolColor)}>
      <div className="absolute top-0 right-0 w-32 h-32 bg-blue-100/20 rounded-full blur-3xl"></div>
      <CardHeader className="pb-3 relative">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="text-4xl transform transition-transform duration-300 hover:scale-110">
              {toolIcon}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-lg text-gray-900">{event.tool_name}</h3>
                {status === 'running' && (
                  <Badge variant="info" className="flex items-center gap-1 animate-pulse-soft">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Running
                  </Badge>
                )}
                {status === 'complete' && !completeEvent?.error && (
                  <Badge variant="success" className="flex items-center gap-1 animate-scaleIn">
                    <CheckCircle2 className="h-3 w-3" />
                    Completed
                  </Badge>
                )}
                {(status === 'error' || completeEvent?.error) && (
                  <Badge variant="error" className="flex items-center gap-1 animate-scaleIn">
                    <XCircle className="h-3 w-3" />
                    Failed
                  </Badge>
                )}
              </div>
              <p className="text-sm text-gray-500 mt-1 font-medium">
                {isComplete && completeEvent?.duration
                  ? `Completed in ${formatDuration(completeEvent.duration)}`
                  : 'Executing...'}
              </p>
            </div>
          </div>
          <button
            onClick={() => setExpanded(!expanded)}
            className="text-gray-400 hover:text-gray-700 transition-all duration-300 hover:scale-110 p-1 rounded-lg hover:bg-gray-100"
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
        <CardContent className="space-y-4 animate-fadeIn relative">
          {/* Arguments */}
          {(startEvent?.arguments || (event as any).arguments) && (
            <div>
              <p className="text-sm font-semibold text-gray-700 mb-2 flex items-center gap-2">
                <span className="w-1.5 h-1.5 bg-blue-500 rounded-full"></span>
                Arguments:
              </p>
              <pre className="bg-gradient-to-br from-gray-50 to-gray-100 p-4 rounded-xl text-xs overflow-x-auto border border-gray-200 shadow-soft font-mono">
                {formatJSON((startEvent?.arguments || (event as any).arguments))}
              </pre>
            </div>
          )}

          {/* Result */}
          {completeEvent?.result && (
            <div>
              <p className="text-sm font-semibold text-gray-700 mb-2 flex items-center gap-2">
                <span className="w-1.5 h-1.5 bg-green-500 rounded-full"></span>
                Result:
              </p>
              <pre className="bg-gradient-to-br from-gray-50 to-gray-100 p-4 rounded-xl text-xs overflow-x-auto border border-gray-200 shadow-soft max-h-96 font-mono">
                {completeEvent.result}
              </pre>
            </div>
          )}

          {/* Error */}
          {completeEvent?.error && (
            <div>
              <p className="text-sm font-semibold text-red-700 mb-2 flex items-center gap-2">
                <span className="w-1.5 h-1.5 bg-red-500 rounded-full animate-pulse"></span>
                Error:
              </p>
              <pre className="bg-gradient-to-br from-red-50 to-red-100 p-4 rounded-xl text-xs overflow-x-auto border border-red-200 shadow-soft font-mono">
                {completeEvent.error}
              </pre>
            </div>
          )}

          {/* Call ID */}
          <div className="text-xs text-gray-400 font-mono pt-2 border-t border-gray-100">
            Call ID: {event.call_id}
          </div>
        </CardContent>
      )}
    </Card>
  );
}
