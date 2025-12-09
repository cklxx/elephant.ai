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
    'workflow.node.output.delta': 'border-border bg-muted',
    'workflow.node.output.summary': 'border-primary/30 bg-primary/10',
    'workflow.tool.started': 'border-primary/30 bg-primary/10',
    'workflow.tool.completed': 'border-emerald-200 bg-emerald-50',
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

// Map of raw tool names to human readable names
const TOOL_NAME_MAP: Record<string, string> = {
  // File Operations
  'file_read': 'Read File',
  'read_resource': 'Read Resource',
  'file_write': 'Write File',
  'write_to_file': 'Write Code',
  'replace_file_content': 'Update Code',
  'multi_replace_file_content': 'Update Multiple Code Blocks',
  'file_edit': 'Edit File',
  'list_dir': 'List Directory',
  'list_files': 'List Files',
  'find_by_name': 'Find File',
  'search_in_file': 'Search in File',
  'grep_search': 'Search Codebase',

  // Code Execution
  'bash': 'Run Shell',
  'run_command': 'Run Command',
  'code_execute': 'Execute Python',
  'python_execute': 'Execute Python',
  'read_terminal': 'Read Terminal',
  'send_command_input': 'Send Input',

  // Web
  'web_search': 'Search Web',
  'search_web': 'Search Web',
  'web_fetch': 'Fetch Page',
  'read_url_content': 'Read Website',
  'open_browser_url': 'Open Browser',
  'read_browser_page': 'Read Page',
  'click_browser_element': 'Click Element',
  'type_browser_element': 'Type Input',
  'scroll_browser_page': 'Scroll Page',

  // Agent/Task
  'task_boundary': 'Update Task',
  'notify_user': 'Notify User',
  'think': 'Thinking',
  'todo_read': 'Check Tasks',
  'todo_update': 'Update Tasks',

  // AI Generation
  'text_to_image': 'Generate Image',
  'video_generate': 'Generate Video',
  'vision_analyze': 'Analyze Image',
};

const TOOL_WORD_REPLACEMENTS: Record<string, string> = {
  'dir': 'Directory',
  'cmd': 'Command',
  'exec': 'Execute',
  'gen': 'Generate',
};

export function humanizeToolName(name: string): string {
  if (!name) return '';
  const normalized = name.toLowerCase().trim();

  // Direct map lookup
  if (TOOL_NAME_MAP[normalized]) {
    return TOOL_NAME_MAP[normalized];
  }

  // Fallback: Replace underscores with spaces and capitalize
  const parts = normalized.split(/[_.]/);
  return parts.map(part => {
    if (TOOL_WORD_REPLACEMENTS[part]) return TOOL_WORD_REPLACEMENTS[part];
    return part.charAt(0).toUpperCase() + part.slice(1);
  }).join(' ');
}
