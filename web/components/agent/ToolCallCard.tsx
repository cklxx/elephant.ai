'use client';

import { Card, CardContent, CardHeader } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { ToolCallStartEvent, ToolCallCompleteEvent } from '@/lib/types';
import { getToolIcon, getToolColor, formatDuration, formatJSON } from '@/lib/utils';
import { isToolCallStartEvent, hasArguments } from '@/lib/typeGuards';
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

  const completeEvent = event.event_type === 'tool_call_complete' ? event : null;
  const startEvent = isToolCallStartEvent(event) ? event : null;
  const isComplete = event.event_type === 'tool_call_complete';

  return (
    <Card className={cn('console-card border-l-4 animate-fadeIn overflow-hidden', toolColor)}>
      <CardHeader className="pb-3">
        <div className="flex items-start justify-between">
          <div className="flex items-center gap-3">
            <div className="text-4xl hover-subtle rounded-md p-1">
              {toolIcon}
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="console-heading text-lg">{event.tool_name}</h3>
                {status === 'running' && (
                  <Badge variant="info" className="flex items-center gap-1 animate-pulse">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    Running
                  </Badge>
                )}
                {status === 'complete' && !completeEvent?.error && (
                  <Badge variant="success" className="flex items-center gap-1 animate-fadeIn">
                    <CheckCircle2 className="h-3 w-3" />
                    Completed
                  </Badge>
                )}
                {(status === 'error' || completeEvent?.error) && (
                  <Badge variant="error" className="flex items-center gap-1 animate-fadeIn">
                    <XCircle className="h-3 w-3" />
                    Failed
                  </Badge>
                )}
              </div>
              <p className="console-caption text-sm mt-1">
                {isComplete && completeEvent?.duration
                  ? `Completed in ${formatDuration(completeEvent.duration)}`
                  : 'Executing...'}
              </p>
            </div>
          </div>
          <button
            onClick={() => setExpanded(!expanded)}
            className="text-muted-foreground hover:text-foreground hover-subtle p-1 rounded-md"
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
        <CardContent className="space-y-4 animate-fadeIn">
          {/* Arguments */}
          {hasArguments(event) && (
            <div>
              <p className="console-subheading text-sm mb-2 flex items-center gap-2">
                <span className="w-1.5 h-1.5 bg-primary rounded-full"></span>
                Arguments:
              </p>
              <pre className="console-card bg-muted p-4 text-xs overflow-x-auto font-mono">
                {formatJSON(event.arguments)}
              </pre>
            </div>
          )}

          {/* Result */}
          {completeEvent?.result && (
            <div>
              <p className="console-subheading text-sm mb-2 flex items-center gap-2">
                <span className="w-1.5 h-1.5 bg-emerald-500 rounded-full"></span>
                Result:
              </p>
              <pre className="console-card bg-muted p-4 text-xs overflow-x-auto max-h-96 font-mono">
                {completeEvent.result}
              </pre>
            </div>
          )}

          {/* Error */}
          {completeEvent?.error && (
            <div>
              <p className="text-sm font-semibold text-destructive mb-2 flex items-center gap-2">
                <span className="w-1.5 h-1.5 bg-destructive rounded-full animate-pulse"></span>
                Error:
              </p>
              <pre className="console-card bg-destructive/5 p-4 text-xs overflow-x-auto border-destructive font-mono">
                {completeEvent.error}
              </pre>
            </div>
          )}

          {/* Call ID */}
          <div className="text-xs console-caption font-mono pt-2 border-t border-border">
            Call ID: {event.call_id}
          </div>
        </CardContent>
      )}
    </Card>
  );
}
