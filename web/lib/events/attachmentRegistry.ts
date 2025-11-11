import {
  AnyAgentEvent,
  AttachmentPayload,
  TaskCompleteEvent,
  ToolCallCompleteEvent,
  UserTaskEvent,
} from "@/lib/types";

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

  clear() {
    this.store = {};
    this.displayedByTool.clear();
  }

  private upsertMany(attachments?: AttachmentMap) {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return;
    }
    Object.entries(normalized).forEach(([key, attachment]) => {
      this.store[key] = attachment;
    });
  }

  private recordToolAttachments(attachments?: AttachmentMap) {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return;
    }
    Object.keys(normalized).forEach((key) => this.displayedByTool.add(key));
    this.upsertMany(normalized);
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
    switch (event.event_type) {
      case "user_task":
        this.upsertMany((event as UserTaskEvent).attachments);
        break;
      case "tool_call_complete":
        this.recordToolAttachments(
          (event as ToolCallCompleteEvent).attachments as
            | AttachmentMap
            | undefined,
        );
        break;
      case "task_complete": {
        const taskEvent = event as TaskCompleteEvent;
        const normalized = this.filterUndisplayed(
          taskEvent.attachments as AttachmentMap | undefined,
        );
        if (normalized) {
          taskEvent.attachments = normalized;
          this.upsertMany(normalized);
          break;
        }
        const fallback = this.filterUndisplayed(
          this.resolveFromContent(taskEvent.final_answer),
        );
        if (fallback) {
          taskEvent.attachments = fallback;
          this.upsertMany(fallback);
        }
        break;
      }
      default:
        break;
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
