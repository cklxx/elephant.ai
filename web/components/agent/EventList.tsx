/**
 * EventList - Virtual scrolling event list
 *
 * Uses @tanstack/react-virtual for optimal performance with large event lists.
 * Features:
 * - Virtual scrolling for thousands of events
 * - Auto-scroll to bottom when new events arrive
 * - Manual scroll lock when user scrolls up
 * - Dynamic row height estimation
 *
 * @example
 * ```tsx
 * <EventList events={events} isConnected={isConnected} />
 * ```
 */

'use client';

import { useRef, useEffect } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { AnyAgentEvent } from '@/lib/types';
import { EventLine } from './EventLine';

interface EventListProps {
  events: AnyAgentEvent[];
  isConnected: boolean;
}

/**
 * EventList - Virtualized event stream display
 * Implements auto-scroll behavior when new events arrive
 */
export function EventList({ events, isConnected }: EventListProps) {
  const parentRef = useRef<HTMLDivElement>(null);
  const isUserScrollingRef = useRef(false);

  // Setup virtualizer with dynamic sizing
  const virtualizer = useVirtualizer({
    count: events.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 35, // Estimated row height in pixels
    overscan: 10, // Render 10 extra items for smoother scrolling
  });

  /**
   * Track user scroll behavior
   * If user scrolls up, disable auto-scroll
   */
  useEffect(() => {
    const parent = parentRef.current;
    if (!parent) return;

    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = parent;
      const isNearBottom = scrollHeight - scrollTop - clientHeight < 100;
      isUserScrollingRef.current = !isNearBottom;
    };

    parent.addEventListener('scroll', handleScroll, { passive: true });
    return () => parent.removeEventListener('scroll', handleScroll);
  }, []);

  /**
   * Auto-scroll to bottom when new events arrive
   * Only scrolls if user hasn't manually scrolled up
   */
  useEffect(() => {
    if (events.length > 0 && !isUserScrollingRef.current) {
      // Scroll to the last item
      virtualizer.scrollToIndex(events.length - 1, {
        align: 'end',
        behavior: 'smooth',
      });
    }
  }, [events.length, virtualizer]);

  return (
    <div className="space-y-2">
      {/* Virtual scrolling container */}
      <div
        ref={parentRef}
        className="font-mono text-xs overflow-auto"
        style={{ maxHeight: 'calc(100vh - 300px)' }}
      >
        <div
          style={{
            height: `${virtualizer.getTotalSize()}px`,
            width: '100%',
            position: 'relative',
          }}
        >
          {virtualizer.getVirtualItems().map((virtualItem) => (
            <div
              key={virtualItem.key}
              data-index={virtualItem.index}
              ref={(node) => {
                if (node) {
                  virtualizer.measureElement(node);
                }
              }}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                transform: `translateY(${virtualItem.start}px)`,
              }}
            >
              <EventLine event={events[virtualItem.index]} />
            </div>
          ))}
        </div>
      </div>

      {/* Active indicator */}
      {isConnected && events.length > 0 && (
        <div className="flex items-center gap-2 text-xs text-muted-foreground pt-2">
          <div className="w-1.5 h-1.5 rounded-full bg-green-500 animate-pulse" />
          <span>Listening for events...</span>
        </div>
      )}
    </div>
  );
}
