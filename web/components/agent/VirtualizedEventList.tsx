'use client';

import { useRef, useEffect } from 'react';
import Image from 'next/image';
import { useVirtualizer } from '@tanstack/react-virtual';
import { AnyAgentEvent } from '@/lib/types';
import { TaskAnalysisCard } from './TaskAnalysisCard';
import { ToolCallCard } from './ToolCallCard';
import { ThinkingIndicator } from './ThinkingIndicator';
import { TaskCompleteCard } from './TaskCompleteCard';
import { ErrorCard } from './ErrorCard';
import { useTranslation } from '@/lib/i18n';

interface VirtualizedEventListProps {
  events: AnyAgentEvent[];
  autoScroll?: boolean;
}

export function VirtualizedEventList({ events, autoScroll = true }: VirtualizedEventListProps) {
  const t = useTranslation();
  const parentRef = useRef<HTMLDivElement>(null);

  // Create virtualizer instance
  const virtualizer = useVirtualizer({
    count: events.length,
    getScrollElement: () => parentRef.current,
    estimateSize: () => 200, // Estimated height per item in pixels
    overscan: 5, // Render 5 extra items above/below viewport
  });

  // Auto-scroll to bottom when new events arrive (with debouncing)
  useEffect(() => {
    if (!autoScroll || events.length === 0) return;

    const timeoutId = setTimeout(() => {
      virtualizer.scrollToIndex(events.length - 1, {
        align: 'end',
        behavior: 'smooth',
      });
    }, 100); // Debounce scroll updates

    return () => clearTimeout(timeoutId);
  }, [events.length, autoScroll, virtualizer]);

  return (
    <div
      ref={parentRef}
      className="min-h-[400px] max-h-[800px] overflow-y-auto pr-2 scroll-smooth"
      style={{
        // Add subtle scrollbar styling
        scrollbarWidth: 'thin',
        scrollbarColor: '#cbd5e1 #f1f5f9',
      }}
    >
      {events.length === 0 ? (
        <div className="glass-card flex flex-col items-center justify-center h-64 text-gray-500 rounded-xl shadow-soft animate-fadeIn">
          <div className="w-16 h-16 rounded-full bg-gradient-to-br from-gray-100 to-gray-200 flex items-center justify-center mb-4 animate-pulse-soft">
            <span className="text-2xl">💭</span>
          </div>
          <p className="font-medium">{t('events.empty')}</p>
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
                <div className="pb-4">
                  <EventCard event={event} />
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function EventCard({ event }: { event: AnyAgentEvent }) {
  const t = useTranslation();
  switch (event.event_type) {
    case 'task_analysis':
      return <TaskAnalysisCard event={event} />;

    case 'thinking':
      return <ThinkingIndicator />;

    case 'tool_call_start':
      return <ToolCallCard event={event} status="running" />;

    case 'tool_call_complete':
      const hasError = 'error' in event && event.error;
      return (
        <ToolCallCard
          event={event}
          status={hasError ? 'error' : 'complete'}
        />
      );

    case 'task_complete':
      return <TaskCompleteCard event={event} />;

    case 'error':
      return <ErrorCard event={event} />;

    case 'iteration_start':
      return (
        <div className="flex items-center gap-2 text-sm text-foreground font-medium px-4 py-2 bg-primary/10 rounded-lg border-l-2 border-primary/50 animate-slideIn">
          <span className="w-2 h-2 bg-primary rounded-full animate-pulse"></span>
          <span>
            {t('events.iteration.progress', {
              iteration: event.iteration,
              total: event.total_iters,
            })}
          </span>
        </div>
      );

    case 'iteration_complete':
      return (
        <div className="flex items-center justify-end gap-3 text-xs text-gray-500 font-medium px-4 py-2 bg-gradient-to-l from-gray-50/50 to-transparent rounded-lg border-r-2 border-gray-300 animate-fadeIn">
          <span className="flex items-center gap-1">
            <span className="w-1.5 h-1.5 bg-emerald-500 rounded-full"></span>
            {t('events.iteration.complete', { iteration: event.iteration })}
          </span>
          <span>•</span>
          <span className="px-2 py-0.5 bg-primary/10 text-primary rounded border border-primary/30">
            {t('events.iteration.tokens', { count: event.tokens_used })}
          </span>
          <span className="px-2 py-0.5 bg-muted text-foreground rounded border border-border">
            {t('events.iteration.tools', { count: event.tools_run })}
          </span>
        </div>
      );

    // New event types (backend not yet emitting, but ready for when they do)
    case 'research_plan':
      return (
        <div className="glass-card p-4 rounded-xl shadow-soft border-l-4 border-primary/40 animate-slideIn">
          <h3 className="text-sm font-semibold text-gray-700 mb-2 flex items-center gap-2">
            <span className="text-lg">🔬</span>
            {t('events.researchPlan.title', { count: event.estimated_iterations })}
          </h3>
          <ol className="space-y-2 ml-4">
            {event.plan_steps.map((step, idx) => (
              <li key={idx} className="text-sm text-gray-600 flex items-start gap-2">
                <span className="font-semibold text-primary">{idx + 1}.</span>
                <span>{step}</span>
              </li>
            ))}
          </ol>
        </div>
      );

    case 'step_started':
      return (
        <div className="flex items-center gap-2 text-sm text-primary font-medium px-4 py-2 bg-primary/10 rounded-lg border-l-2 border-primary/40 animate-slideIn">
          <span className="w-2 h-2 bg-primary rounded-full animate-pulse"></span>
          <span>
            {t('events.step.started', {
              index: event.step_index + 1,
              description: event.step_description,
            })}
          </span>
        </div>
      );

    case 'step_completed':
      return (
        <div className="glass-card p-4 rounded-xl shadow-soft border-l-4 border-green-400 animate-slideIn">
          <h3 className="text-sm font-semibold text-emerald-700 mb-2 flex items-center gap-2">
            <span className="text-lg">✅</span>
            {t('events.step.completed', { index: event.step_index + 1 })}
          </h3>
          <p className="text-sm text-gray-600 ml-6">{event.step_result}</p>
        </div>
      );

    case 'browser_snapshot':
      return (
        <div className="glass-card p-4 rounded-xl shadow-soft border-l-4 border-primary/40 animate-slideIn">
          <h3 className="text-sm font-semibold text-gray-700 mb-2 flex items-center gap-2">
            <span className="text-lg">🌐</span>
            {t('events.browserSnapshot.title')}
          </h3>
          <p className="text-xs text-gray-500 font-mono mb-2">{event.url}</p>
          {event.screenshot_data && (
            <div className="relative w-full overflow-hidden rounded-lg border border-gray-200 shadow-sm">
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
            <details className="mt-2">
              <summary className="text-xs text-gray-600 cursor-pointer hover:text-gray-900">
                {t('events.browserSnapshot.preview')}
              </summary>
              <pre className="mt-2 bg-gray-50 p-2 rounded text-xs overflow-x-auto max-h-40">
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
