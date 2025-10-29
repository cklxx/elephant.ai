'use client';

import { useRef, useEffect, useState, useMemo, useCallback, useId } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { AnyAgentEvent, ToolCallStartEvent } from '@/lib/types';
import { TaskAnalysisCard } from './TaskAnalysisCard';
import { ToolCallCard } from './ToolCallCard';
import { ThinkingIndicator } from './ThinkingIndicator';
import { TaskCompleteCard } from './TaskCompleteCard';
import { ErrorCard } from './ErrorCard';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';

interface VirtualizedEventListProps {
  events: AnyAgentEvent[];
  autoScroll?: boolean;
  focusedEventIndex?: number | null;
  onJumpToLatest?: () => void;
  className?: string;
}

export function VirtualizedEventList({
  events,
  autoScroll = true,
  focusedEventIndex = null,
  onJumpToLatest,
  className,
}: VirtualizedEventListProps) {
  const t = useTranslation();
  const parentRef = useRef<HTMLDivElement>(null);
  const isAutoScrollingRef = useRef(false);
  const [isPinnedToLatest, setIsPinnedToLatest] = useState(true);
  const [liveMessage, setLiveMessage] = useState('');
  const previousCountRef = useRef(events.length);
  const descriptionId = useId();

  // Create virtualizer instance
  const virtualizer = useVirtualizer({
    count: events.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 200, // Estimated height per item in pixels
    overscan: 5, // Render 5 extra items above/below viewport
  });

  // Track whether the user has scrolled away from the bottom
  useEffect(() => {
    const scrollElement = parentRef.current;
    if (!scrollElement) return;

    const handleScroll = () => {
      if (isAutoScrollingRef.current) {
        return;
      }

      const { scrollTop, scrollHeight, clientHeight } = scrollElement;
      const distanceFromBottom = scrollHeight - (scrollTop + clientHeight);
      const isNearBottom = distanceFromBottom <= 48;
      setIsPinnedToLatest(isNearBottom);
    };

    scrollElement.addEventListener('scroll', handleScroll, { passive: true });
    return () => {
      scrollElement.removeEventListener('scroll', handleScroll);
    };
  }, []);

  const scrollToLatest = useCallback(
    (behavior: ScrollBehavior = 'smooth') => {
      if (!parentRef.current || events.length === 0) {
        return;
      }

      isAutoScrollingRef.current = true;
      virtualizer.scrollToIndex(events.length - 1, {
        align: 'end',
        behavior: behavior as 'auto' | 'smooth' | undefined,
      });

      const timeout = setTimeout(() => {
        isAutoScrollingRef.current = false;
        setIsPinnedToLatest(true);
      }, behavior === 'auto' ? 0 : 160);

      return () => {
        clearTimeout(timeout);
        isAutoScrollingRef.current = false;
      };
    },
    [events.length, virtualizer]
  );

  // Auto-scroll to bottom when new events arrive if the user hasn't scrolled away
  useEffect(() => {
    if (!autoScroll || events.length === 0 || !isPinnedToLatest) {
      return;
    }

    const cancel = scrollToLatest('smooth');
    return () => {
      if (typeof cancel === 'function') {
        cancel();
      }
    };
  }, [events.length, autoScroll, isPinnedToLatest, scrollToLatest]);

  useEffect(() => {
    if (!parentRef.current) return;
    if (autoScroll && events.length > 0) {
      return scrollToLatest('auto') || undefined;
    }
  }, [autoScroll, events.length, scrollToLatest]);

  useEffect(() => {
    if (
      focusedEventIndex == null ||
      Number.isNaN(focusedEventIndex) ||
      focusedEventIndex < 0 ||
      focusedEventIndex >= events.length
    ) {
      return;
    }

    const isLatest = focusedEventIndex >= events.length - 1;
    setIsPinnedToLatest(isLatest);

    isAutoScrollingRef.current = true;
    virtualizer.scrollToIndex(focusedEventIndex, {
      align: isLatest ? 'end' : 'center',
      behavior: 'smooth',
    });

    const timeout = setTimeout(() => {
      isAutoScrollingRef.current = false;
    }, 200);

    return () => {
      clearTimeout(timeout);
      isAutoScrollingRef.current = false;
    };
  }, [focusedEventIndex, events.length, virtualizer]);

  useEffect(() => {
    if (events.length <= previousCountRef.current) {
      previousCountRef.current = events.length;
      return;
    }

    const diff = events.length - previousCountRef.current;
    previousCountRef.current = events.length;
    const message =
      diff === 1
        ? t('events.stream.newEventSingular')
        : t('events.stream.newEventPlural', { count: diff });
    setLiveMessage(message);

    const timeout = setTimeout(() => setLiveMessage(''), 2000);
    return () => clearTimeout(timeout);
  }, [events.length, t]);

  const toolCallStartEvents = useMemo(() => {
    const map = new Map<string, ToolCallStartEvent>();
    for (const event of events) {
      if (event.event_type === 'tool_call_start') {
        map.set(event.call_id, event);
      }
    }
    return map;
  }, [events]);

  return (
    <div
      className={cn(
        'relative overflow-hidden rounded-[28px] border-4 border-border bg-card/95 shadow-[14px_14px_0_rgba(0,0,0,0.72)]',
        className,
      )}
    >
      <span id={descriptionId} className="sr-only">
        {t('events.stream.ariaDescription')}
      </span>
      <div
        ref={parentRef}
        className="console-scrollbar min-h-[420px] max-h-[820px] overflow-y-auto px-5 pb-10 pt-8 scroll-smooth"
        role="log"
        aria-live="polite"
        aria-relevant="additions"
        aria-atomic="false"
        aria-label={t('events.stream.ariaLabel')}
        aria-describedby={descriptionId}
        style={{
          // Add subtle scrollbar styling
          scrollbarWidth: 'thin',
          scrollbarColor: '#cbd5e1 #f1f5f9',
        }}
      >
        {events.length === 0 ? (
          <div className="console-empty-state h-[320px]">
            <span className="text-3xl" aria-hidden>
              💭
            </span>
            <p className="text-sm font-semibold uppercase tracking-[0.22em] text-foreground">
              {t('events.emptyTitle')}
            </p>
            <p className="console-microcopy max-w-xs">{t('events.emptyHint')}</p>
          </div>
        ) : (
          <div
            style={{
              height: `${virtualizer.getTotalSize()}px`,
              width: '100%',
              position: 'relative',
            }}
          >
            {virtualizer.getVirtualItems().map((virtualItem) => {
              const event = events[virtualItem.index];
              const isFocused = focusedEventIndex === virtualItem.index;
              return (
                <div
                  key={virtualItem.key}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    transform: `translateY(${virtualItem.start}px)`,
                  }}
                  data-index={virtualItem.index}
                  ref={virtualizer.measureElement}
                >
                  <div
                    className={cn('pb-6', isFocused && 'scroll-mt-40')}
                    data-focused={isFocused ? 'true' : undefined}
                  >
                    <div
                      className={cn(
                        'console-card bg-card/98 px-5 py-4 transition-all duration-150 ease-out',
                        'hover:-translate-y-1 hover:-translate-x-1 hover:shadow-[12px_12px_0_rgba(0,0,0,0.68)]',
                        isFocused &&
                          'outline outline-2 outline-offset-4 outline-foreground shadow-[14px_14px_0_rgba(0,0,0,0.68)]',
                      )}
                    >
                      <EventCard
                        event={event}
                        pairedStart={
                          event.event_type === 'tool_call_complete'
                            ? toolCallStartEvents.get(event.call_id)
                            : undefined
                        }
                        isFocused={isFocused}
                      />
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {!isPinnedToLatest && events.length > 0 && (
        <button
          type="button"
          onClick={() => {
            scrollToLatest('smooth');
            onJumpToLatest?.();
          }}
          className="console-button console-button-secondary absolute bottom-5 right-5 inline-flex items-center gap-2 px-4 py-1.5 text-[11px] uppercase"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
          {t('events.scrollToLatest') ?? 'View latest'}
        </button>
      )}
      {liveMessage && (
        <div aria-live="polite" className="sr-only">
          {liveMessage}
        </div>
      )}
    </div>
  );
}

function EventCard({
  event,
  pairedStart,
  isFocused = false,
}: {
  event: AnyAgentEvent;
  pairedStart?: ToolCallStartEvent;
  isFocused?: boolean;
}) {
  const t = useTranslation();
  switch (event.event_type) {
    case 'task_analysis':
      return <TaskAnalysisCard event={event} />;

    case 'thinking':
      return <ThinkingIndicator />;

    case 'tool_call_start':
      return <ToolCallCard event={event} status="running" pairedStart={pairedStart} isFocused={isFocused} />;

    case 'tool_call_complete':
      const hasError = 'error' in event && event.error;
      return (
        <ToolCallCard
          event={event}
          status={hasError ? 'error' : 'complete'}
          pairedStart={pairedStart}
          isFocused={isFocused}
        />
      );

    case 'task_complete':
      return <TaskCompleteCard event={event} />;

    case 'error':
      return <ErrorCard event={event} />;

    case 'iteration_start':
      return (
        <div className="flex items-center gap-3">
          <span className="inline-flex h-3 w-3 animate-pulse rounded-full bg-foreground" />
          <span className="text-sm font-semibold uppercase tracking-[0.24em] text-foreground">
            {t('events.iteration.progress', {
              iteration: event.iteration,
              total: event.total_iters,
            })}
          </span>
        </div>
      );

    case 'iteration_complete':
      return (
        <div className="flex flex-wrap items-center gap-3">
          <span className="console-quiet-chip text-xs uppercase">
            {t('events.iteration.complete', { iteration: event.iteration })}
          </span>
          <span className="console-microcopy uppercase tracking-[0.24em] text-muted-foreground">
            {t('events.iteration.tokens', { count: event.tokens_used })}
          </span>
          <span className="console-microcopy uppercase tracking-[0.24em] text-muted-foreground">
            {t('events.iteration.tools', { count: event.tools_run })}
          </span>
        </div>
      );

    // New event types (backend not yet emitting, but ready for when they do)
    case 'research_plan':
      return (
        <div className="space-y-3">
          <h3 className="text-sm font-semibold uppercase tracking-[0.2em] text-foreground">
            {t('events.researchPlan.title', { count: event.estimated_iterations })}
          </h3>
          <ol className="list-decimal space-y-1 pl-5 text-sm text-foreground/75">
            {event.plan_steps.map((step, idx) => (
              <li key={idx}>{step}</li>
            ))}
          </ol>
        </div>
      );

    case 'step_started':
      return (
        <div className="flex items-center gap-3">
          <span className="inline-flex h-3 w-3 animate-pulse rounded-full bg-foreground" />
          <span className="text-sm font-semibold uppercase tracking-[0.24em] text-foreground">
            {t('events.step.started', {
              index: event.step_index + 1,
              description: event.step_description,
            })}
          </span>
        </div>
      );

    case 'step_completed':
      return (
        <div className="space-y-2">
          <p className="text-sm font-semibold uppercase tracking-[0.24em] text-foreground">
            {t('events.step.completed', { index: event.step_index + 1 })}
          </p>
          <p className="text-sm text-foreground/75">{event.step_result}</p>
        </div>
      );

    case 'browser_info': {
      const details: Array<[string, string]> = [];
      if (typeof event.success === 'boolean') {
        details.push([
          t('events.browserInfo.statusLabel'),
          event.success ? t('events.browserInfo.statusAvailable') : t('events.browserInfo.statusUnavailable'),
        ]);
      }
      if (event.message) {
        details.push([t('events.browserInfo.messageLabel'), event.message]);
      }
      if (event.user_agent) {
        details.push([t('events.browserInfo.userAgentLabel'), event.user_agent]);
      }
      if (event.cdp_url) {
        details.push([t('events.browserInfo.cdpLabel'), event.cdp_url]);
      }
      if (event.vnc_url) {
        details.push([t('events.browserInfo.vncLabel'), event.vnc_url]);
      }
      if (event.viewport_width && event.viewport_height) {
        details.push([
          t('events.browserInfo.viewportLabel'),
          `${event.viewport_width} × ${event.viewport_height}`,
        ]);
      }

      return (
        <div className="space-y-3">
          <h3 className="text-sm font-semibold uppercase tracking-[0.2em] text-foreground">{t('events.browserInfo.title')}</h3>
          <p className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
            {t('events.browserInfo.captured', {
              timestamp: new Date(event.captured).toLocaleString(),
            })}
          </p>
          {details.length > 0 ? (
            <dl className="space-y-2 text-sm text-foreground/80">
              {details.map(([label, value]) => (
                <div key={label} className="flex flex-col rounded-lg border border-border bg-background/90 px-3 py-2 shadow-[4px_4px_0_rgba(0,0,0,0.35)]">
                  <dt className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">{label}</dt>
                  <dd className="break-words text-sm text-foreground">{value}</dd>
                </div>
              ))}
            </dl>
          ) : (
            <p className="console-microcopy text-muted-foreground">{t('events.browserInfo.noData')}</p>
          )}
        </div>
      );
    }

    default:
      return null;
  }
}
