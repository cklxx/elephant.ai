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
      <div className="flex items-end gap-2">
        <div className="flex-1 relative">
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
            className="w-full rounded-lg border border-border/60 bg-background/80 px-3 py-2 text-sm font-mono text-foreground shadow-sm transition focus:outline-none focus:ring-2 focus:ring-primary/40 disabled:cursor-not-allowed disabled:opacity-50 min-h-[2.75rem] max-h-32 resize-none overflow-y-auto"
            style={{ fieldSizing: 'content' } as any}
          />
        </div>

        <button
          type="submit"
          disabled={disabled || loading || !task.trim()}
          className="flex h-10 flex-shrink-0 items-center justify-center gap-1.5 rounded-full bg-primary px-4 py-2 text-sm font-semibold uppercase tracking-wide text-primary-foreground shadow-sm transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
          title={loading ? 'Running...' : 'Submit (Enter)'}
          data-testid="task-submit"
        >
          {loading ? (
            <span className="flex items-center gap-1.5">
              <span className="w-1 h-1 rounded-full bg-current animate-pulse" />
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

      <div className="mt-2 text-[11px] font-mono uppercase tracking-wide text-muted-foreground/70">
        Enter to send · Shift+Enter for new line
      </div>
    </form>
  );
}
