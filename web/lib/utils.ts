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
  if (toolName.startsWith('file_') || toolName === 'list_files') {
    return 'text-blue-600 border-blue-200 bg-blue-50';
  }
  if (toolName === 'bash' || toolName === 'code_execute') {
    return 'text-purple-600 border-purple-200 bg-purple-50';
  }
  if (toolName.includes('grep') || toolName === 'find') {
    return 'text-green-600 border-green-200 bg-green-50';
  }
  if (toolName.startsWith('web_')) {
    return 'text-orange-600 border-orange-200 bg-orange-50';
  }
  if (toolName === 'think') {
    return 'text-gray-600 border-gray-200 bg-gray-50';
  }
  if (toolName.startsWith('todo_')) {
    return 'text-cyan-600 border-cyan-200 bg-cyan-50';
  }
  return 'text-gray-600 border-gray-200 bg-gray-50';
}

// Get event card style based on event type
export function getEventCardStyle(eventType: string): string {
  const styleMap: Record<string, string> = {
    task_analysis: 'border-purple-200 bg-purple-50',
    thinking: 'border-gray-200 bg-gray-50',
    think_complete: 'border-blue-200 bg-blue-50',
    tool_call_start: 'border-blue-200 bg-blue-50',
    tool_call_complete: 'border-green-200 bg-green-50',
    task_complete: 'border-green-300 bg-green-50',
    error: 'border-red-200 bg-red-50',
  };
  return styleMap[eventType] || 'border-gray-200 bg-gray-50';
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
