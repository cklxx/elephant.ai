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
      <div className="relative flex items-center gap-3">
        <div className="flex-1 relative group">
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
              "w-full px-4 py-3 text-sm rounded-xl border-2 border-gray-200",
              "bg-white/80 backdrop-blur-sm",
              "focus:outline-none focus:ring-2 focus:ring-blue-500/50 focus:border-blue-500",
              "transition-all duration-300 ease-out",
              "resize-none",
              "disabled:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-60",
              "placeholder:text-gray-400",
              "shadow-soft hover:shadow-medium",
              "group-hover:border-gray-300"
            )}
          />
          {!loading && task.trim() && (
            <div className="absolute bottom-2 right-2 text-xs text-gray-400 animate-fadeIn">
              {task.length} chars
            </div>
          )}
        </div>
        <Button
          type="submit"
          disabled={disabled || loading || !task.trim()}
          size="lg"
          className={cn(
            "self-end px-6 py-3 rounded-xl",
            "bg-gradient-to-r from-blue-600 to-purple-600",
            "hover:from-blue-700 hover:to-purple-700",
            "transition-all duration-300 ease-out",
            "shadow-medium hover:shadow-strong",
            "hover:scale-105 active:scale-95",
            "disabled:opacity-50 disabled:cursor-not-allowed disabled:hover:scale-100"
          )}
        >
          {loading ? (
            <div className="flex items-center gap-2">
              <Loader2 className="h-4 w-4 animate-spin" />
              <span>Running...</span>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <Send className="h-4 w-4" />
              <span>Execute</span>
            </div>
          )}
        </Button>
      </div>
      <div className="mt-3 flex items-center justify-between">
        <p className="text-xs text-gray-500">
          Press <kbd className="px-1.5 py-0.5 bg-gray-100 border border-gray-300 rounded text-xs font-mono">Enter</kbd> to submit,
          <kbd className="ml-1 px-1.5 py-0.5 bg-gray-100 border border-gray-300 rounded text-xs font-mono">Shift+Enter</kbd> for new line
        </p>
      </div>
    </form>
  );
}
