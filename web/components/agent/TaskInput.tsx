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
    <form onSubmit={handleSubmit} className="w-full">
      <div className="flex gap-2 items-end">
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
            className="w-full px-3 py-2 text-sm border border-border/50 rounded bg-background resize-none focus:outline-none focus:border-primary disabled:opacity-50 disabled:cursor-not-allowed font-mono min-h-[2.5rem] max-h-32 overflow-y-auto"
            style={{ fieldSizing: 'content' } as any}
          />
        </div>

        <button
          type="submit"
          disabled={disabled || loading || !task.trim()}
          className="flex-shrink-0 px-3 py-2 bg-primary text-primary-foreground rounded hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-all text-sm font-medium h-10"
          title={loading ? 'Running...' : 'Submit (Enter)'}
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

      <div className="mt-1.5 text-xs text-muted-foreground/60 font-mono">
        Enter to send · Shift+Enter for new line
      </div>
    </form>
  );
}
