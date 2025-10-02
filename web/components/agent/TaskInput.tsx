'use client';

import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Send, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';

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
  placeholder = "Enter your task... (e.g., 'Add a dark mode toggle to the settings page')",
}: TaskInputProps) {
  const [task, setTask] = useState('');

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (task.trim() && !loading) {
      onSubmit(task.trim());
      setTask('');
    }
  };

  return (
    <form onSubmit={handleSubmit} className="w-full">
      <div className="relative flex items-center gap-2">
        <textarea
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
          rows={3}
          className={cn(
            "flex-1 w-full px-4 py-3 text-sm rounded-lg border border-gray-300",
            "focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent",
            "resize-none",
            "disabled:bg-gray-100 disabled:cursor-not-allowed",
            "placeholder:text-gray-400"
          )}
        />
        <Button
          type="submit"
          disabled={disabled || loading || !task.trim()}
          size="lg"
          className="self-end"
        >
          {loading ? (
            <>
              <Loader2 className="mr-2 h-4 w-4 animate-spin" />
              Running...
            </>
          ) : (
            <>
              <Send className="mr-2 h-4 w-4" />
              Execute
            </>
          )}
        </Button>
      </div>
      <p className="mt-2 text-xs text-gray-500">
        Press Enter to submit, Shift+Enter for new line
      </p>
    </form>
  );
}
