'use client';

import { useState, useRef, useEffect } from 'react';
import { Send } from 'lucide-react';

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
  placeholder = "Describe your task...",
}: TaskInputProps) {
  const [task, setTask] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

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
      <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:gap-3">
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
            placeholder={placeholder}
            disabled={disabled || loading}
            rows={1}
            aria-label="Task input"
            data-testid="task-input"
            className="w-full rounded-2xl border border-slate-200/80 bg-white/90 px-4 py-3 text-sm text-slate-700 shadow-sm transition focus:border-sky-300 focus:outline-none focus:ring-2 focus:ring-sky-200 disabled:cursor-not-allowed disabled:opacity-50 min-h-[3rem] max-h-36 resize-none overflow-y-auto"
            style={{ fieldSizing: 'content' } as any}
          />
        </div>

        <button
          type="submit"
          disabled={disabled || loading || !task.trim()}
          className="inline-flex h-12 flex-shrink-0 items-center justify-center gap-2 rounded-2xl bg-sky-500 px-5 text-sm font-semibold text-white shadow-lg shadow-sky-500/30 transition hover:bg-sky-600 disabled:cursor-not-allowed disabled:bg-slate-200 disabled:text-slate-400"
          title={loading ? 'Running...' : 'Submit (Enter)'}
          data-testid="task-submit"
        >
          {loading ? (
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-white/80 animate-pulse" />
              Running
            </span>
          ) : (
            <span className="flex items-center gap-1.5">
              <Send className="h-3.5 w-3.5" />
              Send
            </span>
          )}
        </button>
      </div>

      <div className="text-[11px] font-medium uppercase tracking-[0.3em] text-slate-400">
        Enter 发送 · Shift+Enter 换行
      </div>
    </form>
  );
}
