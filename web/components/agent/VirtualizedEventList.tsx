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
import { ReactNode } from 'react';

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
  const { visibleEvents, indexMap } = useMemo(() => {
    const filtered: AnyAgentEvent[] = [];
    const mapping = new Map<number, number>();
    let lastTaskAnalysisIndex = -1;

    for (let index = events.length - 1; index >= 0; index -= 1) {
      if (events[index]?.event_type === 'task_analysis') {
        lastTaskAnalysisIndex = index;
        break;
      }
    }

    events.forEach((event, index) => {
      if (event.event_type === 'task_analysis' && index !== lastTaskAnalysisIndex) {
        return;
      }

      mapping.set(index, filtered.length);
      filtered.push(event);
    });

    return { visibleEvents: filtered, indexMap: mapping };
  }, [events]);

  const parentRef = useRef<HTMLDivElement>(null);
  const isAutoScrollingRef = useRef(false);
  const [isPinnedToLatest, setIsPinnedToLatest] = useState(true);
  const [liveMessage, setLiveMessage] = useState('');
  const previousSnapshotRef = useRef<{
    count: number;
    lastEvent: AnyAgentEvent | null;
  }>({
    count: visibleEvents.length,
    lastEvent: visibleEvents.length > 0 ? visibleEvents[visibleEvents.length - 1] : null,
  });
  const descriptionId = useId();
  const effectiveFocusedEventIndex = useMemo(() => {
    if (focusedEventIndex == null) {
      return null;
    }

    return indexMap.get(focusedEventIndex) ?? null;
  }, [focusedEventIndex, indexMap]);

  // Create virtualizer instance
  const virtualizer = useVirtualizer({
    count: visibleEvents.length,
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
      if (!parentRef.current || visibleEvents.length === 0) {
        return;
      }

      isAutoScrollingRef.current = true;
      virtualizer.scrollToIndex(visibleEvents.length - 1, {
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
    [visibleEvents.length, virtualizer]
  );

  // Auto-scroll to bottom when new events arrive if the user hasn't scrolled away
  useEffect(() => {
    if (!autoScroll || visibleEvents.length === 0 || !isPinnedToLatest) {
      return;
    }

    const cancel = scrollToLatest('smooth');
    return () => {
      if (typeof cancel === 'function') {
        cancel();
      }
    };
  }, [visibleEvents.length, autoScroll, isPinnedToLatest, scrollToLatest]);

  useEffect(() => {
    if (!parentRef.current) return;
    if (autoScroll && visibleEvents.length > 0) {
      return scrollToLatest('auto') || undefined;
    }
  }, [autoScroll, visibleEvents.length, scrollToLatest]);

  useEffect(() => {
    if (
      effectiveFocusedEventIndex == null ||
      Number.isNaN(effectiveFocusedEventIndex) ||
      effectiveFocusedEventIndex < 0 ||
      effectiveFocusedEventIndex >= visibleEvents.length
    ) {
      return;
    }

    const isLatest = effectiveFocusedEventIndex >= visibleEvents.length - 1;
    setIsPinnedToLatest(isLatest);

    isAutoScrollingRef.current = true;
    virtualizer.scrollToIndex(effectiveFocusedEventIndex, {
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
  }, [effectiveFocusedEventIndex, visibleEvents.length, virtualizer]);

  useEffect(() => {
    const previous = previousSnapshotRef.current;
    const lastEvent = visibleEvents.length > 0 ? visibleEvents[visibleEvents.length - 1] : null;

    let diff = visibleEvents.length - previous.count;
    if (diff <= 0 && lastEvent && lastEvent !== previous.lastEvent) {
      diff = 1;
    }

    previousSnapshotRef.current = {
      count: visibleEvents.length,
      lastEvent,
    };

    if (diff <= 0) {
      return;
    }

    const message =
      diff === 1
        ? t('events.stream.newEventSingular')
        : t('events.stream.newEventPlural', { count: diff });
    setLiveMessage(message);

    const timeout = setTimeout(() => setLiveMessage(''), 2000);
    return () => clearTimeout(timeout);
  }, [visibleEvents, t]);

  const toolCallStartEvents = useMemo(() => {
    const map = new Map<string, ToolCallStartEvent>();
    for (const event of visibleEvents) {
      if (event.event_type === 'tool_call_start') {
        map.set(event.call_id, event);
      }
    }
    return map;
  }, [visibleEvents]);

  return (
    <div
      className={cn(
        'relative overflow-hidden rounded-[28px] border-4 border-border bg-card/95',
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
        {visibleEvents.length === 0 ? (
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
              const event = visibleEvents[virtualItem.index];
              const isFocused = effectiveFocusedEventIndex === virtualItem.index;
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
                        'hover:-translate-y-1 hover:-translate-x-1',
                        isFocused && 'outline outline-2 outline-offset-4 outline-foreground',
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

      {!isPinnedToLatest && visibleEvents.length > 0 && (
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

  const wrapWithContext = (content: ReactNode) => (
    <div className="space-y-3">
      <EventContextMeta event={event} />
      {content}
    </div>
  );

  switch (event.event_type) {
    case 'task_analysis':
      return wrapWithContext(<TaskAnalysisCard event={event} />);

    case 'thinking':
      return <ThinkingIndicator />;

    case 'tool_call_start':
      return wrapWithContext(
        <ToolCallCard event={event} status="running" pairedStart={pairedStart} isFocused={isFocused} />
      );

    case 'tool_call_complete':
      const hasError = 'error' in event && event.error;
      return wrapWithContext(
        <ToolCallCard
          event={event}
          status={hasError ? 'error' : 'done'}
          pairedStart={pairedStart}
          isFocused={isFocused}
        />
      );

    case 'task_complete':
      return wrapWithContext(<TaskCompleteCard event={event} />);

    case 'error':
      return wrapWithContext(<ErrorCard event={event} />);

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
      return wrapWithContext(
        <div className="flex flex-wrap items-center gap-3">
          <span className="console-quiet-chip text-xs uppercase">
            {t('events.iteration.complete', { iteration: event.iteration })}
          </span>
          <span className="console-microcopy uppercase tracking-[0.24em] text-muted-foreground">
            {t('events.iteration.tokens', { count: event.tokens_used })}
          </span>
        </div>
      );

    // New event types (backend not yet emitting, but ready for when they do)
    case 'research_plan':
      return wrapWithContext(
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
      return wrapWithContext(
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
                <div key={label} className="flex flex-col rounded-lg border border-border bg-background/90 px-3 py-2">
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

function EventContextMeta({ event }: { event: AnyAgentEvent }) {
  const parts: string[] = [];
  if (event.session_id) {
    parts.push(`Session ${event.session_id}`);
  }
  if (event.task_id) {
    parts.push(`Task ${event.task_id}`);
  }
  if (event.parent_task_id) {
    parts.push(`Parent ${event.parent_task_id}`);
  }

  if (parts.length === 0) {
    return null;
  }

  return (
    <p className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
      {parts.join(' · ')}
    </p>
  );
}
