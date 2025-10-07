// EventLine component - renders a single agent event
// Optimized with React.memo for virtual scrolling performance

import React from 'react';
import { AnyAgentEvent } from '@/lib/types';
import { formatContent, formatTimestamp } from './formatters';
import { getEventStyle } from './styles';

interface EventLineProps {
  event: AnyAgentEvent;
}

/**
 * EventLine - Single event display component
 * Memoized for optimal virtual scrolling performance
 */
export const EventLine = React.memo(function EventLine({ event }: EventLineProps) {
  const timestamp = formatTimestamp(event.timestamp);
  const content = formatContent(event);
  const style = getEventStyle(event);

  return (
    <div className="flex gap-3 group hover:bg-muted/30 -mx-2 px-2 py-1 rounded transition-colors">
      <span className="text-muted-foreground/50 flex-shrink-0 select-none">
        {timestamp}
      </span>
      <span className={style}>
        {content}
      </span>
    </div>
  );
});
