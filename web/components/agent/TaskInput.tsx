'use client';

import { useState, useRef, useEffect } from 'react';
import { Send } from 'lucide-react';
import { useTranslation } from '@/lib/i18n';

interface TaskInputProps {
  onSubmit: (task: string) => void;
  disabled?: boolean;
  loading?: boolean;
  placeholder?: string;
}

export function TaskInput({
  onSubmit,
  disabled = false,
  loading = false,
  placeholder,
}: TaskInputProps) {
  const [task, setTask] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const t = useTranslation();
  const resolvedPlaceholder = placeholder ?? t('console.input.placeholder.idle');

  // Auto-resize textarea
  useEffect(() => {
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.style.height = textareaRef.current.scrollHeight + 'px';
    }
  }, [task]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (task.trim() && !loading && !disabled) {
      onSubmit(task.trim());
      setTask('');
    }
  };

  return (
    <form onSubmit={handleSubmit} className="w-full" data-testid="task-input-form">
      <div className="flex flex-col gap-2.5 sm:flex-row sm:items-end sm:gap-2.5">
        <div className="relative flex-1">
          <textarea
            ref={textareaRef}
            value={task}
            onChange={(e) => setTask(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                handleSubmit(e);
              }
            }}
            placeholder={resolvedPlaceholder}
            disabled={disabled || loading}
            rows={1}
            aria-label={t('task.input.ariaLabel')}
            data-testid="task-input"
            className="min-h-[2.75rem] max-h-32 w-full resize-none overflow-y-auto rounded-2xl border border-slate-200/80 bg-white/90 px-3.5 py-2.5 text-[13px] text-slate-700 shadow-sm transition focus:border-sky-300 focus:outline-none focus:ring-2 focus:ring-sky-200 disabled:cursor-not-allowed disabled:opacity-50"
            style={{ fieldSizing: 'content' } as any}
          />
        </div>

        <button
          type="submit"
          disabled={disabled || loading || !task.trim()}
          className="inline-flex h-[2.75rem] flex-shrink-0 items-center justify-center gap-2 rounded-2xl bg-sky-500 px-4 text-sm font-semibold text-white shadow-lg shadow-sky-500/30 transition hover:bg-sky-600 disabled:cursor-not-allowed disabled:bg-slate-200 disabled:text-slate-400"
          title={loading ? t('task.submit.title.running') : t('task.submit.title.default')}
          data-testid="task-submit"
        >
          {loading ? (
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-white/80 animate-pulse" />
              {t('task.submit.running')}
            </span>
          ) : (
            <span className="flex items-center gap-1.5">
              <Send className="h-3.5 w-3.5" />
              {t('task.submit.label')}
            </span>
          )}
        </button>
      </div>

      <div className="mt-1 flex justify-end text-[10px] font-medium uppercase tracking-[0.35em] text-slate-300">
        {t('console.input.hotkeyHint')}
      </div>
    </form>
  );
}
