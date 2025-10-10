'use client';

import { useRef, useEffect, useState, useMemo, useCallback, useId } from 'react';
import Image from 'next/image';
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
}

export function VirtualizedEventList({
  events,
  autoScroll = true,
  focusedEventIndex = null,
  onJumpToLatest,
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
        behavior,
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
    <div className="relative">
      <span id={descriptionId} className="sr-only">
        {t('events.stream.ariaDescription')}
      </span>
      <div
        ref={parentRef}
        className="min-h-[400px] max-h-[800px] overflow-y-auto pr-2 scroll-smooth"
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
          <div className="flex h-60 flex-col items-center justify-center gap-2 text-center text-slate-500">
            <span className="text-2xl" aria-hidden>
              💭
            </span>
            <p className="text-sm font-semibold text-slate-600">{t('events.emptyTitle')}</p>
            <p className="text-xs text-slate-400">{t('events.emptyHint')}</p>
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
                    className={cn('pb-4', isFocused && 'scroll-mt-32')}
                    data-focused={isFocused ? 'true' : undefined}
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
          className="absolute bottom-4 right-4 inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white/90 px-3 py-1.5 text-xs font-medium text-slate-600 shadow-lg shadow-slate-900/5 transition hover:border-slate-300 hover:bg-white"
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
        <div className="relative pl-4 text-lg font-semibold leading-tight text-slate-700">
          <span className="absolute left-0 top-2 h-2 w-2 rounded-full bg-primary/70 animate-pulse" />
          {t('events.iteration.progress', {
            iteration: event.iteration,
            total: event.total_iters,
          })}
        </div>
      );

    case 'iteration_complete':
      return (
        <div className="flex flex-wrap items-center gap-3 text-sm text-slate-500">
          <span className="inline-flex items-center gap-2 text-base font-semibold text-slate-700">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
            {t('events.iteration.complete', { iteration: event.iteration })}
          </span>
          <span className="text-xs font-medium uppercase tracking-[0.25em] text-slate-400">
            {t('events.iteration.tokens', { count: event.tokens_used })}
          </span>
          <span className="text-xs font-medium uppercase tracking-[0.25em] text-slate-400">
            {t('events.iteration.tools', { count: event.tools_run })}
          </span>
        </div>
      );

    // New event types (backend not yet emitting, but ready for when they do)
    case 'research_plan':
      return (
        <div className="space-y-2">
          <h3 className="text-sm font-semibold text-slate-800">
            {t('events.researchPlan.title', { count: event.estimated_iterations })}
          </h3>
          <ol className="space-y-1 pl-4 text-sm text-slate-600 list-decimal">
            {event.plan_steps.map((step, idx) => (
              <li key={idx}>{step}</li>
            ))}
          </ol>
        </div>
      );

    case 'step_started':
      return (
        <div className="relative pl-4 text-base font-semibold leading-tight text-slate-700">
          <span className="absolute left-0 top-2 h-2 w-2 rounded-full bg-primary/60 animate-pulse" />
          {t('events.step.started', {
            index: event.step_index + 1,
            description: event.step_description,
          })}
        </div>
      );

    case 'step_completed':
      return (
        <div className="space-y-2">
          <p className="text-lg font-semibold leading-tight text-emerald-600">
            {t('events.step.completed', { index: event.step_index + 1 })}
          </p>
          <p className="text-sm text-slate-600">{event.step_result}</p>
        </div>
      );

    case 'browser_snapshot':
      return (
        <div className="space-y-2">
          <h3 className="text-sm font-semibold text-slate-800">{t('events.browserSnapshot.title')}</h3>
          <p className="text-xs font-mono text-slate-400">{event.url}</p>
          {event.screenshot_data && (
            <div className="relative w-full overflow-hidden rounded-lg">
              <Image
                src={`data:image/png;base64,${event.screenshot_data}`}
                alt={t('events.browserSnapshot.alt')}
                width={1280}
                height={720}
                className="h-auto w-full"
                unoptimized
                sizes="(max-width: 768px) 100vw, 720px"
              />
            </div>
          )}
          {event.html_preview && (
            <details className="text-xs text-slate-500">
              <summary className="cursor-pointer hover:text-slate-700">
                {t('events.browserSnapshot.preview')}
              </summary>
              <pre className="mt-2 max-h-40 overflow-x-auto whitespace-pre-wrap rounded bg-slate-100 p-2 text-xs text-slate-600">
                {event.html_preview}
              </pre>
            </details>
          )}
        </div>
      );

    default:
      return null;
  }
}
