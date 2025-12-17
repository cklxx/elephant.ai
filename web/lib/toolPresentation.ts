import { AttachmentPayload } from '@/lib/types';

export function stripSystemReminders(content: string): string {
  if (!content) return '';
  if (!content.includes('<system-reminder>')) {
    return content.trim();
  }

  const lines = content.split('\n');
  const filtered: string[] = [];
  let inReminder = false;

  for (const line of lines) {
    const trimmed = line.trim();
    if (trimmed.startsWith('<system-reminder>')) {
      inReminder = true;
      if (trimmed.endsWith('</system-reminder>')) {
        inReminder = false;
      }
      continue;
    }
    if (trimmed.endsWith('</system-reminder>')) {
      inReminder = false;
      continue;
    }
    if (!inReminder) {
      filtered.push(line);
    }
  }

  return filtered.join('\n').trim();
}

function getAttachmentNames(attachments?: Record<string, AttachmentPayload> | null): string[] {
  if (!attachments) return [];
  return Object.keys(attachments).filter((name) => typeof name === 'string' && name.trim().length > 0);
}

function summarizeAttachmentNames(attachments?: Record<string, AttachmentPayload> | null): string | undefined {
  const names = getAttachmentNames(attachments);
  if (names.length === 0) return undefined;
  if (names.length === 1) return names[0];
  if (names.length === 2) return `${names[0]}、${names[1]}`;
  return `${names[0]}、${names[1]} 等 ${names.length} 个`;
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}

function getTodoCounts(metadata?: Record<string, any> | null): {
  total?: number;
  inProgress?: number;
  pending?: number;
  completed?: number;
} | null {
  if (!metadata || typeof metadata !== 'object') return null;

  const total = metadata.total_count;
  const inProgress = metadata.in_progress_count;
  const pending = metadata.pending_count;
  const completed = metadata.completed_count;

  if (
    isFiniteNumber(total) ||
    isFiniteNumber(inProgress) ||
    isFiniteNumber(pending) ||
    isFiniteNumber(completed)
  ) {
    return {
      total: isFiniteNumber(total) ? total : undefined,
      inProgress: isFiniteNumber(inProgress) ? inProgress : undefined,
      pending: isFiniteNumber(pending) ? pending : undefined,
      completed: isFiniteNumber(completed) ? completed : undefined,
    };
  }

  return null;
}

export function userFacingToolSummary(input: {
  toolName: string;
  result?: string | null;
  error?: string | null;
  metadata?: Record<string, any> | null;
  attachments?: Record<string, AttachmentPayload> | null;
}): string | undefined {
  const tool = input.toolName.toLowerCase().trim();

  if (input.error && input.error.trim()) {
    return input.error.trim().length > 100
      ? `${input.error.trim().slice(0, 100)}…`
      : input.error.trim();
  }

  if (tool === 'think') {
    return '内部处理';
  }

  if (tool === 'todo_update') {
    const counts = getTodoCounts(input.metadata);
    if (counts) {
      const parts: string[] = [];
      if (typeof counts.total === 'number') parts.push(`共 ${counts.total} 项`);
      if (typeof counts.inProgress === 'number') parts.push(`进行中 ${counts.inProgress}`);
      if (typeof counts.pending === 'number') parts.push(`待办 ${counts.pending}`);
      if (typeof counts.completed === 'number') parts.push(`已完成 ${counts.completed}`);
      if (parts.length > 0) {
        return `待办已更新（${parts.join(' / ')}）`;
      }
    }
    return '待办已更新';
  }

  if (tool === 'artifacts_write') {
    const names = summarizeAttachmentNames(input.attachments);
    if (names) {
      return `已生成文件：${names}`;
    }

    const sanitized = stripSystemReminders(input.result ?? '');
    const match = sanitized.match(/^Saved\s+(.+?)\s+\((.+?)\)\s*$/i);
    if (match) {
      return `已生成文件：${match[1]}`;
    }
  }

  const sanitized = stripSystemReminders(input.result ?? '');
  if (!sanitized) return undefined;
  return sanitized.length > 100 ? `${sanitized.slice(0, 100)}…` : sanitized;
}

export function userFacingToolResultText(input: {
  toolName?: string | null;
  result?: string | null;
  metadata?: Record<string, any> | null;
  attachments?: Record<string, AttachmentPayload> | null;
}): string {
  const tool = (input.toolName ?? '').toLowerCase().trim();
  const sanitized = stripSystemReminders(input.result ?? '');

  if (tool === 'think') {
    return '';
  }

  if (tool === 'artifacts_write') {
    const names = summarizeAttachmentNames(input.attachments);
    if (names) {
      return `已生成文件：${names}`;
    }
    const match = sanitized.match(/^Saved\s+(.+?)\s+\((.+?)\)\s*$/i);
    if (match) {
      return `已生成文件：${match[1]}`;
    }
  }

  if (tool === 'todo_update') {
    // Keep the task list, but strip internal reminders.
    return sanitized;
  }

  return sanitized;
}
