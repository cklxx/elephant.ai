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

  clear() {
    this.store = {};
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
        this.upsertMany(
          (event as ToolCallCompleteEvent).attachments as
            | AttachmentMap
            | undefined,
        );
        break;
      case "task_complete": {
        const taskEvent = event as TaskCompleteEvent;
        const normalized = normalizeAttachmentMap(
          taskEvent.attachments as AttachmentMap | undefined,
        );
        if (normalized) {
          taskEvent.attachments = normalized;
          this.upsertMany(normalized);
          break;
        }
        const fallback = this.resolveFromContent(taskEvent.final_answer);
        if (fallback) {
          taskEvent.attachments = fallback;
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
