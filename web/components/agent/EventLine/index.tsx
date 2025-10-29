// EventLine component - renders a single agent event
// Optimized with React.memo for virtual scrolling performance

import React from 'react';
import { AnyAgentEvent, ToolCallCompleteEvent } from '@/lib/types';
import { formatContent, formatTimestamp } from './formatters';
import { getEventStyle } from './styles';
import { ToolOutputCard } from '../ToolOutputCard';
import { cn } from '@/lib/utils';

interface EventLineProps {
  event: AnyAgentEvent;
}

/**
 * EventLine - Single event display component
 * Memoized for optimal virtual scrolling performance
 */
export const EventLine = React.memo(function EventLine({ event }: EventLineProps) {
  if (event.event_type === 'tool_call_complete') {
    const completeEvent = event as ToolCallCompleteEvent & {
      arguments?: Record<string, unknown>;
    };

    return (
      <ToolOutputCard
        toolName={completeEvent.tool_name}
        parameters={completeEvent.arguments}
        result={completeEvent.result}
        error={completeEvent.error}
        duration={completeEvent.duration}
        timestamp={completeEvent.timestamp}
        callId={completeEvent.call_id}
      />
    );
  }

  const timestamp = formatTimestamp(event.timestamp);
  const content = formatContent(event);
  const style = getEventStyle(event);

  return (
    <div className={cn('console-event-line', style.line)}>
      <span className="console-event-timestamp">{timestamp}</span>
      <div className={cn('console-event-content', style.content)}>{content}</div>
    </div>
  );
});
