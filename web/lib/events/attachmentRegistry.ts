import {
  AnyAgentEvent,
  AttachmentExportStatusEvent,
  AttachmentPayload,
  TaskCompleteEvent,
  ToolCallCompleteEvent,
  UserTaskEvent,
} from "@/lib/types";
import {
  getTaskAttachments,
  saveTaskAttachments,
} from "@/lib/stores/taskStore";

const PLACEHOLDER_PATTERN = /\[([^\[\]]+)\]/g;

type AttachmentMap = Record<string, AttachmentPayload>;

function normalizeAttachmentMap(
  map?: AttachmentMap,
): AttachmentMap | undefined {
  if (!map) {
    return undefined;
  }
  const normalized: AttachmentMap = {};
  for (const [key, attachment] of Object.entries(map)) {
    const normalizedKey = (key || attachment.name || "").trim();
    if (!normalizedKey) {
      continue;
    }
    normalized[normalizedKey] = {
      ...attachment,
      name: attachment.name?.trim() || normalizedKey,
    };
  }
  return Object.keys(normalized).length > 0 ? normalized : undefined;
}

class AttachmentRegistry {
  private store: AttachmentMap = {};
  private displayedByTool = new Set<string>();
  private currentTaskId?: string;

  clear() {
    this.store = {};
    this.displayedByTool.clear();
    this.currentTaskId = undefined;
  }

  private ensureTaskContext(taskId?: string) {
    if (!taskId || this.currentTaskId === taskId) {
      return;
    }
    this.currentTaskId = taskId;
    const cached = getTaskAttachments(taskId);
    if (cached) {
      this.store = cached;
      this.displayedByTool = new Set(Object.keys(cached));
      return;
    }
    this.store = {};
    this.displayedByTool.clear();
  }

  private persist(taskId?: string) {
    if (!taskId || Object.keys(this.store).length === 0) {
      return;
    }
    saveTaskAttachments(taskId, this.store);
  }

  private upsertMany(attachments?: AttachmentMap) {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return false;
    }
    Object.entries(normalized).forEach(([key, attachment]) => {
      this.store[key] = attachment;
    });
    return true;
  }

  private recordToolAttachments(attachments?: AttachmentMap) {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return false;
    }
    Object.keys(normalized).forEach((key) => this.displayedByTool.add(key));
    this.upsertMany(normalized);
    return true;
  }

  private filterUndisplayed(attachments?: AttachmentMap): AttachmentMap | undefined {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return undefined;
    }
    const filteredEntries = Object.entries(normalized).filter(
      ([key]) => !this.displayedByTool.has(key),
    );
    if (filteredEntries.length === 0) {
      return undefined;
    }
    return Object.fromEntries(filteredEntries);
  }

  private resolveFromContent(content: string): AttachmentMap | undefined {
    if (!content || !content.includes("[")) {
      return undefined;
    }
    const resolved: AttachmentMap = {};
    let match: RegExpExecArray | null;
    while ((match = PLACEHOLDER_PATTERN.exec(content)) !== null) {
      const name = match[1]?.trim();
      if (!name || resolved[name]) {
        continue;
      }
      const attachment = this.store[name];
      if (attachment) {
        resolved[name] = attachment;
      }
    }
    return Object.keys(resolved).length > 0 ? resolved : undefined;
  }

  handleEvent(event: AnyAgentEvent) {
    const taskId = event.task_id;
    this.ensureTaskContext(taskId);
    let changed = false;
    switch (event.event_type) {
      case "user_task":
        changed = this.upsertMany((event as UserTaskEvent).attachments) || changed;
        break;
      case "tool_call_complete":
        changed =
          this.recordToolAttachments(
            (event as ToolCallCompleteEvent).attachments as
              | AttachmentMap
              | undefined,
          ) || changed;
        break;
      case "task_complete": {
        const taskEvent = event as TaskCompleteEvent;
        const normalized = this.filterUndisplayed(
          taskEvent.attachments as AttachmentMap | undefined,
        );
        if (normalized) {
          taskEvent.attachments = normalized;
          changed = this.upsertMany(normalized) || changed;
          break;
        }
        const fallback = this.filterUndisplayed(
          this.resolveFromContent(taskEvent.final_answer),
        );
        if (fallback) {
          taskEvent.attachments = fallback;
          changed = this.upsertMany(fallback) || changed;
        }
        break;
      }
      case "attachment_export_status":
        changed =
          this.upsertMany(
            (event as AttachmentExportStatusEvent).attachments as
              | AttachmentMap
              | undefined,
          ) || changed;
        break;
      default:
        break;
    }
    if (changed) {
      this.persist(taskId);
    }
  }
}

const attachmentRegistry = new AttachmentRegistry();

export const handleAttachmentEvent = (event: AnyAgentEvent) => {
  attachmentRegistry.handleEvent(event);
};

export const resetAttachmentRegistry = () => {
  attachmentRegistry.clear();
};
