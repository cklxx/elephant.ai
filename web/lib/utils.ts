import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function isBrowser(): boolean {
  return typeof window !== 'undefined';
}

// Format duration in milliseconds to human readable string
export function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`;
  }
  if (ms < 60000) {
    return `${(ms / 1000).toFixed(2)}s`;
  }
  const minutes = Math.floor(ms / 60000);
  const seconds = ((ms % 60000) / 1000).toFixed(0);
  return `${minutes}m ${seconds}s`;
}

// Format timestamp to relative time
export function formatRelativeTime(timestamp: string, locale: string = 'en-US'): string {
  const targetDate = new Date(timestamp);
  if (Number.isNaN(targetDate.getTime())) {
    return '';
  }

  const now = new Date();
  const diffInSeconds = (targetDate.getTime() - now.getTime()) / 1000;

  const divisions = [
    { amount: 60, unit: 'second' },
    { amount: 60, unit: 'minute' },
    { amount: 24, unit: 'hour' },
    { amount: 7, unit: 'day' },
    { amount: 4.34524, unit: 'week' },
    { amount: 12, unit: 'month' },
    { amount: Number.POSITIVE_INFINITY, unit: 'year' },
  ] as const;

  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' });

  let duration = diffInSeconds;
  for (const division of divisions) {
    if (Math.abs(duration) < division.amount) {
      return formatter.format(
        Math.round(duration),
        division.unit as Intl.RelativeTimeFormatUnit
      );
    }
    duration /= division.amount;
  }

  return new Intl.DateTimeFormat(locale).format(targetDate);
}

// Get tool icon based on tool name
export function getToolIcon(toolName: string): string {
  const iconMap: Record<string, string> = {
    file_read: '📖',
    file_write: '✍️',
    file_edit: '✏️',
    list_files: '📂',
    bash: '🔧',
    code_execute: '▶️',
    grep: '🔍',
    ripgrep: '🔎',
    find: '📁',
    web_search: '🌐',
    web_fetch: '🔗',
    think: '💭',
    todo_read: '📋',
    todo_update: '✅',
  };
  return iconMap[toolName] || '⚙️';
}

// Get tool category color
export function getToolColor(toolName: string): string {
  if (toolName === 'think') {
    return 'text-muted-foreground border-border bg-muted';
  }

  if (toolName === 'bash' || toolName === 'code_execute') {
    return 'text-amber-700 border-amber-200 bg-amber-50';
  }

  return 'text-primary border-primary/30 bg-primary/10';
}

// Get event card style based on event type
export function getEventCardStyle(eventType: string): string {
  const styleMap: Record<string, string> = {
    thinking: 'border-border bg-muted',
    'workflow.node.output.delta': 'border-border bg-muted',
    think_complete: 'border-primary/30 bg-primary/10',
    'workflow.node.output.summary': 'border-primary/30 bg-primary/10',
    tool_call_start: 'border-primary/30 bg-primary/10',
    'workflow.tool.started': 'border-primary/30 bg-primary/10',
    tool_call_complete: 'border-emerald-200 bg-emerald-50',
    'workflow.tool.completed': 'border-emerald-200 bg-emerald-50',
    task_complete: 'border-emerald-300 bg-emerald-50',
    'workflow.result.final': 'border-emerald-300 bg-emerald-50',
    error: 'border-destructive/40 bg-destructive/10',
    'workflow.node.failed': 'border-destructive/40 bg-destructive/10',
  };
  return styleMap[eventType] || 'border-border bg-card';
}

// Truncate long text
export function truncate(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return text.substring(0, maxLength) + '...';
}

// Format JSON with syntax highlighting
export function formatJSON(obj: any): string {
  return JSON.stringify(obj, null, 2);
}
