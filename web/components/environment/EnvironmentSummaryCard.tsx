'use client';

import { CheckCircle2, Circle, ClipboardList } from 'lucide-react';
import { SessionEnvironmentPlan } from '@/lib/environmentPlan';
import { getLanguageLocale, useI18n } from '@/lib/i18n';

interface EnvironmentSummaryCardProps {
  plan: SessionEnvironmentPlan;
  onToggleTodo?: (todoId: string, nextValue: boolean) => void;
}

function formatDate(value: string, locale: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return date.toLocaleString(locale, {
    hour12: false,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export function EnvironmentSummaryCard({ plan, onToggleTodo }: EnvironmentSummaryCardProps) {
  const { t, language } = useI18n();
  const locale = getLanguageLocale(language);

  const generatedAt = formatDate(plan.generatedAt, locale);
  const lastUpdatedAt = formatDate(plan.lastUpdatedAt, locale);
  const sandboxLabel =
    plan.sandboxStrategy === 'required'
      ? t('conversation.environment.sandbox.required')
      : t('conversation.environment.sandbox.recommended');

  const tools = plan.toolsUsed.length
    ? plan.toolsUsed.join(', ')
    : t('conversation.environment.tools.none');

  const completedTodos = plan.todos.filter((todo) => todo.completed).length;
  const remainingTodos = Math.max(plan.todos.length - completedTodos, 0);
  const showTodoStatus = plan.todos.length > 0;

  return (
    <section className="mb-4 rounded-lg border border-slate-200 bg-slate-50 p-4 text-sm text-slate-700">
      <header className="mb-2 flex flex-wrap items-center justify-between gap-2 text-[10px] uppercase tracking-[0.3em] text-slate-500">
        <span>{t('conversation.environment.heading')}</span>
        <span className="font-semibold text-slate-700">{sandboxLabel}</span>
      </header>

      <div className="space-y-1.5 text-xs leading-relaxed text-slate-600">
        <p className="text-[10px] uppercase tracking-[0.25em] text-slate-400">
          {t('conversation.environment.assurance')}
        </p>
        {showTodoStatus && (
          <p className="inline-flex items-center gap-1 rounded-md bg-white/70 px-2 py-1 text-[11px] font-medium text-slate-600">
            {remainingTodos === 0 ? (
              <CheckCircle2 className="h-4 w-4 text-emerald-500" aria-hidden />
            ) : (
              <ClipboardList className="h-4 w-4 text-amber-500" aria-hidden />
            )}
            {remainingTodos === 0
              ? t('conversation.environment.todos.allComplete')
              : t('conversation.environment.todos.remaining', { count: remainingTodos })}
          </p>
        )}
        <p>
          {t('conversation.environment.tools.label', { count: plan.toolsUsed.length })}
          <span className="font-medium text-slate-700"> {tools}</span>
        </p>
        <p>
          {t('conversation.environment.generatedAt', { time: generatedAt })}
        </p>
        <p>
          {t('conversation.environment.lastUpdated', { time: lastUpdatedAt })}
        </p>
        <p className="font-medium text-slate-700">{plan.notes}</p>
      </div>

      <div className="mt-3 space-y-1.5 rounded-md border border-slate-200 bg-white p-3 text-xs text-slate-600">
        <p className="text-[11px] font-semibold uppercase tracking-[0.25em] text-slate-500">
          {plan.blueprint.title}
        </p>
        <p className="leading-relaxed text-slate-600">{plan.blueprint.description}</p>
        <p>
          {t('conversation.environment.capabilities')}:{' '}
          <span className="font-medium text-slate-700">
            {plan.blueprint.recommendedCapabilities.join(', ')}
          </span>
        </p>
        <p>
          {t('conversation.environment.persistence')}:{' '}
          <span className="font-medium text-slate-700">{plan.blueprint.persistenceHint}</span>
        </p>
      </div>

      {plan.todos.length > 0 && (
        <div className="mt-3 space-y-2">
          <p className="text-[11px] font-semibold uppercase tracking-[0.25em] text-slate-500">
            {t('conversation.environment.todos.heading')}
          </p>
          <ul className="space-y-2 text-xs leading-relaxed text-slate-600">
            {plan.todos.map((todo) => {
              const isInteractive = Boolean(onToggleTodo);
              const manualBadge =
                todo.manuallySet && t('conversation.environment.todos.manualBadge');

              return (
                <li key={todo.id} className="flex items-start gap-2">
                  <button
                    type="button"
                    onClick={() => onToggleTodo?.(todo.id, !todo.completed)}
                    disabled={!isInteractive}
                    className="mt-0.5 flex h-4 w-4 items-center justify-center rounded-full border border-transparent text-slate-300 transition hover:border-slate-300 hover:text-slate-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-emerald-400 disabled:cursor-default disabled:opacity-60"
                    aria-pressed={todo.completed}
                    aria-label={todo.completed ? t('conversation.environment.todos.uncheck') : t('conversation.environment.todos.check')}
                  >
                    {todo.completed ? (
                      <CheckCircle2 className="h-4 w-4 text-emerald-500" aria-hidden />
                    ) : (
                      <Circle className="h-4 w-4 text-slate-300" aria-hidden />
                    )}
                  </button>
                  <div className="flex-1">
                    <p className={todo.completed ? 'text-slate-400 line-through' : ''}>{todo.label}</p>
                    {manualBadge && (
                      <span className="mt-0.5 inline-flex items-center rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-[0.25em] text-emerald-600">
                        {manualBadge}
                      </span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
        </div>
      )}
    </section>
  );
}
