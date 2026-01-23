'use client';

import { useRef, useEffect, useState, useMemo, useCallback, useId } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { AnyAgentEvent, WorkflowToolStartedEvent } from '@/lib/types';
import { isEventType } from '@/lib/events/matching';
import {
  isWorkflowNodeFailedEvent,
  isWorkflowResultFinalEvent,
  isWorkflowToolCompletedEvent,
  isWorkflowToolStartedEvent,
} from '@/lib/typeGuards';
import { ToolCallCard } from './ToolCallCard';
import { ThinkingIndicator } from './ThinkingIndicator';
import { TaskCompleteCard } from './TaskCompleteCard';
import { ErrorCard } from './ErrorCard';
import { AgentMarkdown } from './AgentMarkdown';
import { useTranslation } from '@/lib/i18n';
import { cn } from '@/lib/utils';
import { Card, CardContent } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { ReactNode } from 'react';
import { isDebugModeEnabled } from '@/lib/debugMode';

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
    const collapsed = collapseFinalResults(events);
    const mapping = new Map<number, number>();
    collapsed.forEach((_, index) => {
      mapping.set(index, index);
    });

    return { visibleEvents: collapsed, indexMap: mapping };
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
  // eslint-disable-next-line react-hooks/incompatible-library -- @tanstack/react-virtual is not React Compiler compatible yet.
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
    const map = new Map<string, WorkflowToolStartedEvent>();
    for (const event of visibleEvents) {
      if (isWorkflowToolStartedEvent(event)) {
        map.set(event.call_id, event);
      }
    }
    return map;
  }, [visibleEvents]);

  return (
    <Card className={cn('relative overflow-hidden', className)}>
      <CardContent className="p-0">
      <span id={descriptionId} className="sr-only">
        {t('events.stream.ariaDescription')}
      </span>
      <div
        ref={parentRef}
        className="min-h-[420px] max-h-[820px] overflow-y-auto px-5 pb-10 pt-8 scroll-smooth"
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
          <div className="flex h-[320px] flex-col items-center justify-center gap-2 text-center">
            <span className="text-3xl" aria-hidden>
              💭
            </span>
            <p className="text-sm font-semibold text-foreground">
              {t('events.emptyTitle')}
            </p>
            <p className="text-xs text-muted-foreground max-w-xs">{t('events.emptyHint')}</p>
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
              const completedEvent = isWorkflowToolCompletedEvent(event) ? event : null;
              const isFocused = effectiveFocusedEventIndex === virtualItem.index;
              const isStreamingDelta = isEventType(event, 'workflow.node.output.delta');
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
                    className={cn(isStreamingDelta ? 'pb-3' : 'pb-6', isFocused && 'scroll-mt-40')}
                    data-focused={isFocused ? 'true' : undefined}
                  >
                    {isStreamingDelta ? (
                      <div
                        className={cn(
                          'px-5 py-2',
                          isFocused && 'outline outline-2 outline-offset-4 outline-foreground',
                        )}
                      >
                        <EventCard
                          event={event}
                          pairedStart={
                            completedEvent
                              ? toolCallStartEvents.get(completedEvent.call_id)
                              : undefined
                          }
                          isFocused={isFocused}
                        />
                      </div>
                    ) : (
                      <Card
                        className={cn(
                          'bg-card/95 transition-all duration-150 ease-out',
                          isFocused && 'outline outline-2 outline-offset-4 outline-foreground',
                        )}
                      >
                        <CardContent className="px-5 py-4">
                          <EventCard
                            event={event}
                            pairedStart={
                              completedEvent
                                ? toolCallStartEvents.get(completedEvent.call_id)
                                : undefined
                            }
                            isFocused={isFocused}
                          />
                        </CardContent>
                      </Card>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {!isPinnedToLatest && visibleEvents.length > 0 && (
        <Button
          type="button"
          onClick={() => {
            scrollToLatest('smooth');
            onJumpToLatest?.();
          }}
          className="absolute bottom-5 right-5 inline-flex items-center gap-2 text-[11px] font-semibold"
          size="sm"
          variant="secondary"
        >
          <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
          {t('events.scrollToLatest') ?? 'View latest'}
        </Button>
      )}
      {liveMessage && (
        <div aria-live="polite" className="sr-only">
          {liveMessage}
        </div>
      )}
      </CardContent>
    </Card>
  );
}

function EventCard({
  event,
  pairedStart,
  isFocused = false,
}: {
  event: AnyAgentEvent;
  pairedStart?: WorkflowToolStartedEvent;
  isFocused?: boolean;
}) {
  const t = useTranslation();

  const wrapWithContext = (content: ReactNode) => (
    <div className="flex flex-col gap-3">
      <EventContextMeta event={event} />
      {content}
    </div>
  );

  if (isEventType(event, 'workflow.node.output.delta')) {
    const delta = (event as any).delta;
    if (typeof delta === 'string' && delta.trim().length > 0) {
      const streamFinished = (event as any).final === true;
      const isStreaming = !streamFinished;
      return (
        <AgentMarkdown
          content={delta}
          className="prose max-w-none text-sm leading-snug text-foreground"
          isStreaming={isStreaming}
          streamFinished={streamFinished}
        />
      );
    }
    return <ThinkingIndicator />;
  }

  if (isWorkflowToolStartedEvent(event)) {
    return wrapWithContext(
      <ToolCallCard event={event} status="running" pairedStart={pairedStart} isFocused={isFocused} />,
    );
  }

  if (isWorkflowToolCompletedEvent(event)) {
    const hasError = Boolean(event.error);
    return wrapWithContext(
      <ToolCallCard
        event={event}
        status={hasError ? 'error' : 'done'}
        pairedStart={pairedStart}
        isFocused={isFocused}
      />,
    );
  }

  if (isWorkflowResultFinalEvent(event)) {
    return wrapWithContext(<TaskCompleteCard event={event} />);
  }

  if (isWorkflowNodeFailedEvent(event)) {
    return wrapWithContext(<ErrorCard event={event} />);
  }

  if (isEventType(event, 'workflow.node.started') && typeof (event as any).iteration === 'number') {
    return (
      <div className="flex items-center gap-3">
        <span className="inline-flex h-3 w-3 animate-pulse rounded-full bg-foreground" />
        <span className="text-sm font-semibold text-foreground">
          {t('events.iteration.progress', {
            iteration: (event as any).iteration,
            total: (event as any).total_iters,
          })}
        </span>
      </div>
    );
  }

  if (isEventType(event, 'workflow.node.completed') && typeof (event as any).iteration === 'number') {
    return wrapWithContext(
      <div className="flex flex-wrap items-center gap-3 text-xs text-foreground">
        <Badge variant="outline">
          {t('events.iteration.complete', { iteration: (event as any).iteration })}
        </Badge>
        <span className="text-muted-foreground">
          {t('events.iteration.tokens', { count: (event as any).tokens_used })}
        </span>
      </div>,
    );
  }

  if (isEventType(event, 'workflow.node.started') && typeof (event as any).step_index === 'number') {
    return wrapWithContext(
      <div className="flex items-center gap-3">
        <span className="inline-flex h-3 w-3 animate-pulse rounded-full bg-foreground" />
        <span className="text-sm font-semibold text-foreground">
          {t('events.step.started', {
            index: (event as any).step_index + 1,
            description: (event as any).step_description,
          })}
        </span>
      </div>,
    );
  }

  if (isEventType(event, 'workflow.node.completed') && typeof (event as any).step_index === 'number') {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-sm font-semibold text-foreground">
          {t('events.step.completed', { index: (event as any).step_index + 1 })}
        </p>
        <p className="text-sm text-foreground/75">{(event as any).step_result}</p>
      </div>
    );
  }

  return null;
}

function EventContextMeta({ event }: { event: AnyAgentEvent }) {
  const debugMode = useMemo(() => isDebugModeEnabled(), []);
  if (!debugMode) {
    return null;
  }

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
    <p className="text-[11px] text-muted-foreground">
      {parts.join(' · ')}
    </p>
  );
}

function collapseFinalResults(events: AnyAgentEvent[]): AnyAgentEvent[] {
  const latestByTask = new Map<string, AnyAgentEvent>();
  const ordered: AnyAgentEvent[] = [];

  for (let i = events.length - 1; i >= 0; i -= 1) {
    const evt = events[i];
    if (isEventType(evt, 'workflow.result.final') && 'task_id' in evt && 'session_id' in evt) {
      const key = `${evt.session_id}|${evt.task_id}`;
      if (latestByTask.has(key)) {
        continue;
      }
      latestByTask.set(key, evt);
      ordered.push(evt);
      continue;
    }
    ordered.push(evt);
  }

  return ordered.reverse();
}
