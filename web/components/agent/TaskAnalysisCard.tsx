'use client';

import { TaskAnalysisEvent } from '@/lib/types';
import { isFallbackActionName } from '@/lib/taskAnalysis';
import { Target, Lightbulb, ListTodo, Search, CheckCircle2 } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

interface TaskAnalysisCardProps {
  event: TaskAnalysisEvent;
}

export function TaskAnalysisCard({ event }: TaskAnalysisCardProps) {
  const t = useTranslation();

  const showActionName = Boolean(event.action_name && !isFallbackActionName(event.action_name));

  return (
    <section className="space-y-4" data-testid="task-analysis-event">
      <header className="flex flex-wrap items-center gap-3 text-foreground">
        <span className="inline-flex h-9 w-9 items-center justify-center rounded-full border-2 border-border bg-card">
          <Target className="h-4 w-4" aria-hidden />
        </span>
        <h3 className="text-lg font-semibold uppercase tracking-[0.2em]">
          {t('events.taskAnalysis.title')}
        </h3>
        <span className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
          {t('events.taskAnalysis.label')}
        </span>
      </header>

      {showActionName && (
        <p className="text-base font-semibold leading-snug text-foreground">
          {event.action_name}
        </p>
      )}

      {event.goal && (
        <div className="space-y-2">
          <p className="console-microcopy font-semibold uppercase tracking-[0.28em] text-muted-foreground">
            {t('events.taskAnalysis.goal')}
          </p>
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/80">
            {event.goal}
          </p>
        </div>
      )}

      {event.approach && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-muted-foreground">
            <Lightbulb className="h-4 w-4" aria-hidden />
            <p className="console-microcopy font-semibold uppercase tracking-[0.28em]">
              {t('events.taskAnalysis.approach')}
            </p>
          </div>
          <p className="whitespace-pre-wrap text-sm leading-relaxed text-foreground/80">
            {event.approach}
          </p>
        </div>
      )}

      {event.success_criteria && event.success_criteria.length > 0 && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-muted-foreground">
            <CheckCircle2 className="h-4 w-4" aria-hidden />
            <p className="console-microcopy font-semibold uppercase tracking-[0.28em]">
              {t('events.taskAnalysis.successCriteria')}
            </p>
          </div>
          <ul className="space-y-2">
            {event.success_criteria.map((criterion, index) => (
              <li key={`${criterion}-${index}`} className="flex items-start gap-2 text-sm text-foreground/80">
                <span className="mt-0.5 inline-flex h-4 w-4 items-center justify-center text-primary">
                  <CheckCircle2 className="h-4 w-4" aria-hidden />
                </span>
                <span>{criterion}</span>
              </li>
            ))}
          </ul>
        </div>
      )}

      {event.steps && event.steps.length > 0 && (
        <div className="space-y-3">
          <div className="flex items-center gap-2 text-muted-foreground">
            <ListTodo className="h-4 w-4" aria-hidden />
            <p className="console-microcopy font-semibold uppercase tracking-[0.28em]">
              {t('events.taskAnalysis.plan')}
            </p>
          </div>
          <ol className="space-y-3">
            {event.steps.map((step, index) => (
              <li
                key={`${step.description}-${index}`}
                className="rounded-lg border border-border/40 bg-card/40 p-3 shadow-sm"
              >
                <div className="flex items-start gap-3">
                  <span className="console-quiet-chip text-xs uppercase text-muted-foreground">
                    {t('events.taskAnalysis.stepNumber', { index: index + 1 })}
                  </span>
                  <div className="space-y-2">
                    <p className="text-sm font-semibold text-foreground">{step.description}</p>
                    {step.rationale && (
                      <p className="text-xs leading-relaxed text-muted-foreground">{step.rationale}</p>
                    )}
                    {step.needs_external_context && (
                      <span className="console-quiet-chip text-[10px] uppercase tracking-[0.3em] text-primary">
                        {t('events.taskAnalysis.requiresContext')}
                      </span>
                    )}
                  </div>
                </div>
              </li>
            ))}
          </ol>
        </div>
      )}

      {event.retrieval_plan && (
        (() => {
          const retrieval = event.retrieval_plan;
          const hasLocal = retrieval.local_queries && retrieval.local_queries.length > 0;
          const hasSearch = retrieval.search_queries && retrieval.search_queries.length > 0;
          const hasCrawl = retrieval.crawl_urls && retrieval.crawl_urls.length > 0;
          const hasGaps = retrieval.knowledge_gaps && retrieval.knowledge_gaps.length > 0;
          const hasNotes = retrieval.notes && retrieval.notes.trim().length > 0;
          const shouldShow = retrieval.should_retrieve || hasLocal || hasSearch || hasCrawl || hasGaps || hasNotes;
          if (!shouldShow) {
            return null;
          }

          return (
            <div className="space-y-3">
              <div className="flex items-center gap-2 text-muted-foreground">
                <Search className="h-4 w-4" aria-hidden />
                <p className="console-microcopy font-semibold uppercase tracking-[0.28em]">
                  {t('events.taskAnalysis.retrievalPlan')}
                </p>
                {retrieval.should_retrieve && (
                  <span className="console-quiet-chip text-[10px] uppercase tracking-[0.3em] text-primary">
                    {t('events.taskAnalysis.retrievalTrigger')}
                  </span>
                )}
              </div>

              <div className="grid gap-3 text-sm text-foreground/80">
                {hasLocal && (
                  <div>
                    <p className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
                      {t('events.taskAnalysis.localQueries')}
                    </p>
                    <ul className="list-disc space-y-1 pl-5">
                      {retrieval.local_queries!.map((query) => (
                        <li key={`local-${query}`}>{query}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {hasSearch && (
                  <div>
                    <p className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
                      {t('events.taskAnalysis.searchQueries')}
                    </p>
                    <ul className="list-disc space-y-1 pl-5">
                      {retrieval.search_queries!.map((query) => (
                        <li key={`search-${query}`}>{query}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {hasCrawl && (
                  <div>
                    <p className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
                      {t('events.taskAnalysis.crawlTargets')}
                    </p>
                    <ul className="list-disc space-y-1 pl-5">
                      {retrieval.crawl_urls!.map((url) => (
                        <li key={`crawl-${url}`}>{url}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {hasGaps && (
                  <div>
                    <p className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
                      {t('events.taskAnalysis.remainingTodos')}
                    </p>
                    <ul className="list-disc space-y-1 pl-5">
                      {retrieval.knowledge_gaps!.map((gap) => (
                        <li key={`gap-${gap}`}>{gap}</li>
                      ))}
                    </ul>
                  </div>
                )}
                {hasNotes && (
                  <div>
                    <p className="console-microcopy uppercase tracking-[0.28em] text-muted-foreground">
                      {t('events.taskAnalysis.retrievalNotes')}
                    </p>
                    <p className="whitespace-pre-wrap text-sm leading-relaxed">{retrieval.notes}</p>
                  </div>
                )}
              </div>
            </div>
          );
        })()
      )}
    </section>
  );
}
