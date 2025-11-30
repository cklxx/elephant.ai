import {
  AnyAgentEvent,
  AttachmentPayload,
  TaskCompleteEvent,
  ToolCallCompleteEvent,
  UserTaskEvent,
  eventMatches,
} from '@/lib/types';

const PLACEHOLDER_PATTERN = /\[([^\[\]]+)\]/g;

type AttachmentMap = Record<string, AttachmentPayload>;

type AttachmentMutations = {
  replace?: AttachmentMap;
  add?: AttachmentMap;
  update?: AttachmentMap;
  remove?: string[];
};

function parseMutationsPayload(
  value: unknown,
): Record<string, any> | null {
  if (!value) {
    return null;
  }

  if (typeof value === "string") {
    try {
      const parsed = JSON.parse(value);
      if (parsed && typeof parsed === "object") {
        return parsed as Record<string, any>;
      }
    } catch {
      return null;
    }
  }

  if (typeof value === "object") {
    return value as Record<string, any>;
  }

  return null;
}

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

function normalizeAttachmentMutations(
  metadata?: Record<string, any>,
): AttachmentMutations | null {
  if (!metadata) {
    return null;
  }

  const raw =
    metadata.attachment_mutations ||
    metadata.attachments_mutations ||
    metadata.attachmentMutations ||
    metadata.attachmentsMutations;
  const parsed = parseMutationsPayload(raw);
  if (!parsed || typeof parsed !== "object") {
    return null;
  }

  const replace = normalizeAttachmentMap(
    parsed.replace || parsed.snapshot || parsed.catalog,
  );
  const add = normalizeAttachmentMap(parsed.add || parsed.create);
  const update = normalizeAttachmentMap(parsed.update || parsed.upsert);
  const remove = Array.isArray(parsed.remove || parsed.delete)
    ? (parsed.remove || parsed.delete)
        .map((key: unknown) =>
          typeof key === "string" ? key.trim() : String(key || "").trim(),
        )
        .filter((key: string) => Boolean(key))
    : [];

  if (replace || add || update || remove.length > 0) {
    return { replace, add, update, remove };
  }

  return null;
}

function shouldIncludeDisplayedAttachments(taskEvent: TaskCompleteEvent): boolean {
  if (taskEvent.is_streaming === true && taskEvent.stream_finished === false) {
    return false;
  }
  return true;
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

  private removeMany(keys?: string[]) {
    if (!keys || keys.length === 0) {
      return;
    }
    keys.forEach((key) => {
      const normalizedKey = (key || "").trim();
      if (!normalizedKey) {
        return;
      }
      delete this.store[normalizedKey];
      this.displayedByTool.delete(normalizedKey);
    });
  }

  private replaceStore(map?: AttachmentMap) {
    if (!map || Object.keys(map).length === 0) {
      return;
    }
    this.store = { ...map };
    this.displayedByTool.clear();
  }

  private mergeMutations(
    base?: AttachmentMap,
    mutations?: AttachmentMutations | null,
  ): AttachmentMap | undefined {
    const merged: AttachmentMap = { ...(base || {}) };

    if (!mutations) {
      return Object.keys(merged).length > 0 ? merged : undefined;
    }

    if (mutations.replace) {
      Object.assign(merged, mutations.replace);
    }

    if (mutations.add) {
      Object.assign(merged, mutations.add);
    }

    if (mutations.update) {
      Object.assign(merged, mutations.update);
    }

    if (mutations.remove && mutations.remove.length > 0) {
      mutations.remove.forEach((key) => {
        const normalizedKey = (key || "").trim();
        if (normalizedKey) {
          delete merged[normalizedKey];
        }
      });
    }

    return Object.keys(merged).length > 0 ? merged : undefined;
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

  private hydrateFromContent(
    content: string,
    options: { markDisplayed?: boolean; skipDisplayed?: boolean } = {},
  ): AttachmentMap | undefined {
    const resolved = this.resolveFromContent(content);
    if (!resolved) {
      return undefined;
    }

    if (options.skipDisplayed) {
      return this.filterUndisplayed(resolved);
    }

    if (options.markDisplayed) {
      this.recordToolAttachments(resolved);
    } else {
      this.upsertMany(resolved);
    }

    return resolved;
  }

  private takeUndisplayedFromStore(): AttachmentMap | undefined {
    return this.takeFromStore();
  }

  private takeFromStore(options: { includeDisplayed?: boolean } = {}): AttachmentMap | undefined {
    const entries = Object.entries(this.store).filter(
      ([key]) => options.includeDisplayed || !this.displayedByTool.has(key),
    );

    if (entries.length === 0) {
      return undefined;
    }

    const result = Object.fromEntries(entries);
    entries.forEach(([key]) => this.displayedByTool.add(key));
    return result;
  }

  handleEvent(event: AnyAgentEvent) {
    switch (true) {
      case event.event_type === 'user_task':
        this.upsertMany((event as UserTaskEvent).attachments);
        break;
      case eventMatches(event, 'workflow.tool.completed', 'tool_call_complete'):
        const toolEvent = event as ToolCallCompleteEvent;
        const normalizedAttachments = normalizeAttachmentMap(
          toolEvent.attachments as AttachmentMap | undefined,
        );
        const attachmentMutations = normalizeAttachmentMutations(
          toolEvent.metadata,
        );

        if (attachmentMutations?.replace) {
          this.replaceStore(attachmentMutations.replace);
        }

        if (attachmentMutations?.remove?.length) {
          this.removeMany(attachmentMutations.remove);
        }

        if (attachmentMutations?.add) {
          this.upsertMany(attachmentMutations.add);
        }

        if (attachmentMutations?.update) {
          this.upsertMany(attachmentMutations.update);
        }

        const mergedAttachments = this.mergeMutations(
          normalizedAttachments,
          attachmentMutations,
        );

        if (mergedAttachments) {
          toolEvent.attachments = mergedAttachments;
          this.recordToolAttachments(mergedAttachments);
        } else {
          toolEvent.attachments = this.hydrateFromContent(toolEvent.result, {
            markDisplayed: true,
          });
        }
        break;
      case eventMatches(event, 'workflow.result.final', 'task_complete'): {
        const taskEvent = event as TaskCompleteEvent;
        const normalized = this.filterUndisplayed(taskEvent.attachments as AttachmentMap | undefined);
        if (normalized) {
          taskEvent.attachments = normalized;
          this.upsertMany(normalized);
          break;
        }

        const fallback = this.hydrateFromContent(taskEvent.final_answer, {
          skipDisplayed: true,
        });
        if (fallback) {
          taskEvent.attachments = fallback;
          this.upsertMany(fallback);
          break;
        }

        const rendered = this.hydrateFromContent(taskEvent.final_answer);
        if (rendered) {
          taskEvent.attachments = rendered;
        }
        if (!taskEvent.attachments) {
          const allowDisplayed = shouldIncludeDisplayedAttachments(taskEvent);
          taskEvent.attachments = this.takeFromStore({
            includeDisplayed: allowDisplayed,
          });
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
