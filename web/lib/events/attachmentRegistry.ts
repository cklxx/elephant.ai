import {
  AnyAgentEvent,
  AttachmentPayload,
  WorkflowArtifactManifestEvent,
  WorkflowResultFinalEvent,
  WorkflowToolCompletedEvent,
  WorkflowInputReceivedEvent,
} from '@/lib/types';
import { isEventType } from '@/lib/events/matching';

type AttachmentMap = Record<string, AttachmentPayload>;
type AttachmentVisibility = 'default' | 'recalled';

type AttachmentMutations = {
  replace?: AttachmentMap;
  add?: AttachmentMap;
  update?: AttachmentMap;
  remove?: string[];
};

type StoredAttachment = {
  attachment: AttachmentPayload;
  firstSeen: number;
  visibility: AttachmentVisibility;
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

function extractPlaceholderKeys(content?: string): string[] {
  if (!content || !content.includes("[")) {
    return [];
  }

  const pattern = /\[([^\[\]]+)\]/g;
  const matches: string[] = [];
  let match: RegExpExecArray | null;

  while ((match = pattern.exec(content)) !== null) {
    const name = match[1]?.trim();
    if (name) {
      matches.push(name);
    }
  }

  return matches;
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

function shouldIncludeDisplayedAttachments(taskEvent: WorkflowResultFinalEvent): boolean {
  if (taskEvent.is_streaming === true && taskEvent.stream_finished === false) {
    return false;
  }
  return true;
}

class AttachmentRegistry {
  private store: Map<string, StoredAttachment> = new Map();
  private displayedByTool = new Set<string>();

  clear() {
    this.store = new Map();
    this.displayedByTool.clear();
  }

  ingestRecalled(attachments?: AttachmentMap, firstSeen?: number) {
    this.upsertMany(attachments, firstSeen, 'recalled');
  }

  private getEventTimestamp(event: AnyAgentEvent): number {
    const parsed = event && typeof (event as any).timestamp === "string"
      ? Date.parse((event as any).timestamp)
      : Number.NaN;
    return Number.isFinite(parsed) ? parsed : Date.now();
  }

  private upsertMany(
    attachments?: AttachmentMap,
    firstSeen?: number,
    visibility: AttachmentVisibility = 'default',
    preserveExistingVisibility = false,
  ) {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return;
    }
    const seenAt = typeof firstSeen === "number" && Number.isFinite(firstSeen) ? firstSeen : Date.now();
    Object.entries(normalized).forEach(([key, attachment]) => {
      const existing = this.store.get(key);
      const firstSeenAt = existing ? Math.min(existing.firstSeen, seenAt) : seenAt;
      const requestedVisibility: AttachmentVisibility =
        attachment.visibility === 'recalled' ? 'recalled' : visibility;
      const nextVisibility: AttachmentVisibility = preserveExistingVisibility && existing?.visibility
        ? existing.visibility
        : requestedVisibility === 'recalled'
          ? 'recalled'
          : existing?.visibility === 'default'
            ? 'default'
            : requestedVisibility;
      this.store.set(key, { attachment, firstSeen: firstSeenAt, visibility: nextVisibility });
    });
  }

  private recordToolAttachments(
    attachments?: AttachmentMap,
    firstSeen?: number,
    visibility: AttachmentVisibility = 'default',
    preserveExistingVisibility = false,
  ) {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return;
    }
    Object.keys(normalized).forEach((key) => this.displayedByTool.add(key));
    this.upsertMany(normalized, firstSeen, visibility, preserveExistingVisibility);
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
      this.store.delete(normalizedKey);
      this.displayedByTool.delete(normalizedKey);
    });
  }

  private replaceStore(map?: AttachmentMap, firstSeen?: number) {
    if (!map || Object.keys(map).length === 0) {
      return;
    }
    this.store = new Map();
    this.upsertMany(map, firstSeen);
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

  private filterUndisplayed(
    attachments?: AttachmentMap,
  ): AttachmentMap | undefined {
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

  private isAvailable(key: string, cutoff?: number): boolean {
    if (!this.store.has(key)) {
      return false;
    }
    if (typeof cutoff !== "number" || !Number.isFinite(cutoff)) {
      return true;
    }
    const stored = this.store.get(key);
    return Boolean(stored && stored.firstSeen <= cutoff);
  }

  private resolveFromContent(
    content: string,
    cutoff?: number,
  ): AttachmentMap | undefined {
    const referencedKeys = extractPlaceholderKeys(content);
    if (referencedKeys.length === 0) {
      return undefined;
    }
    const resolved: AttachmentMap = {};

    referencedKeys.forEach((name) => {
      if (resolved[name]) {
        return;
      }
      const stored = this.store.get(name);
      if (stored && this.isAvailable(name, cutoff)) {
        resolved[name] = stored.attachment;
      }
    });

    return Object.keys(resolved).length > 0 ? resolved : undefined;
  }

  private hydrateFromContent(
    content: string,
    options: { markDisplayed?: boolean; skipDisplayed?: boolean; timestamp?: number } = {},
  ): AttachmentMap | undefined {
    const resolved = this.resolveFromContent(content, options.timestamp);
    if (!resolved) {
      return undefined;
    }

    if (options.skipDisplayed) {
      return this.filterUndisplayed(resolved);
    }

    if (options.markDisplayed) {
      this.recordToolAttachments(resolved, options.timestamp, 'default', true);
    } else {
      this.upsertMany(resolved, options.timestamp, 'default', true);
    }

    return resolved;
  }

  private takeUndisplayedFromStore(): AttachmentMap | undefined {
    return this.takeFromStore();
  }

  private takeFromStore(options: { includeDisplayed?: boolean; timestamp?: number; includeRecalled?: boolean } = {}): AttachmentMap | undefined {
    const entries = Array.from(this.store.entries()).filter(
      ([key, stored]) =>
        (options.includeDisplayed || !this.displayedByTool.has(key)) &&
        this.isAvailable(key, options.timestamp) &&
        (options.includeRecalled !== false || stored.visibility !== 'recalled'),
    );

    if (entries.length === 0) {
      return undefined;
    }

    const result = Object.fromEntries(entries.map(([key, stored]) => [key, stored.attachment]));
    entries.forEach(([key]) => this.displayedByTool.add(key));
    return result;
  }

  private omitPreviouslyDisplayedUnlessReferenced(
    attachments?: AttachmentMap,
    content?: string,
    displayedSnapshot: Set<string> = this.displayedByTool,
  ): AttachmentMap | undefined {
    const normalized = normalizeAttachmentMap(attachments);
    if (!normalized) {
      return undefined;
    }

    const referencedKeys = new Set(extractPlaceholderKeys(content));
    const entries = Object.entries(normalized).filter(([key]) => {
      if (referencedKeys.has(key)) {
        return true;
      }
      return !displayedSnapshot.has(key);
    });

    if (entries.length === 0) {
      return undefined;
    }

    return Object.fromEntries(entries);
  }

  handleEvent(event: AnyAgentEvent) {
    const eventTimestamp = this.getEventTimestamp(event);
    switch (true) {
      case event.event_type === 'workflow.input.received': {
        this.upsertMany((event as WorkflowInputReceivedEvent).attachments ?? undefined, eventTimestamp);
        break;
      }
      case isEventType(event, 'workflow.tool.completed'): {
        const toolEvent = event as WorkflowToolCompletedEvent;
        const normalizedAttachments = normalizeAttachmentMap(
          toolEvent.attachments as AttachmentMap | undefined,
        );
        const attachmentMutations = normalizeAttachmentMutations(
          toolEvent.metadata,
        );

        if (attachmentMutations?.replace) {
          this.replaceStore(attachmentMutations.replace, eventTimestamp);
        }

        if (attachmentMutations?.remove?.length) {
          this.removeMany(attachmentMutations.remove);
        }

        if (attachmentMutations?.add) {
          this.upsertMany(attachmentMutations.add, eventTimestamp);
        }

        if (attachmentMutations?.update) {
          this.upsertMany(attachmentMutations.update, eventTimestamp);
        }

        const mergedAttachments = this.mergeMutations(
          normalizedAttachments,
          attachmentMutations,
        );

        if (mergedAttachments) {
          toolEvent.attachments = mergedAttachments;
          this.recordToolAttachments(mergedAttachments, eventTimestamp);
        } else {
          const content = normalizeAttachmentContent(toolEvent.result);
          toolEvent.attachments = this.hydrateFromContent(content, {
            markDisplayed: true,
            timestamp: eventTimestamp,
          });
        }
        break;
      }
      case isEventType(event, 'workflow.artifact.manifest'): {
        const manifestEvent = event as WorkflowArtifactManifestEvent;
        const payload =
          manifestEvent.payload &&
          typeof manifestEvent.payload === 'object' &&
          !Array.isArray(manifestEvent.payload)
            ? (manifestEvent.payload as Record<string, any>)
            : null;
        const manifest =
          manifestEvent.manifest ??
          (payload?.manifest as Record<string, any> | undefined) ??
          payload;
        const attachments = normalizeAttachmentMap(
          (manifestEvent.attachments as AttachmentMap | undefined) ??
            (payload?.attachments as AttachmentMap | undefined) ??
            (manifest && typeof manifest === 'object'
              ? (manifest as Record<string, any>).attachments
              : undefined),
        );
        if (attachments) {
          this.recordToolAttachments(attachments, eventTimestamp);
          this.upsertMany(attachments, eventTimestamp);
        }
        break;
      }
      case isEventType(event, 'workflow.result.final'): {
        const taskEvent = event as WorkflowResultFinalEvent;
        const displayedSnapshot = new Set(this.displayedByTool);
        const normalized = normalizeAttachmentMap(taskEvent.attachments as AttachmentMap | undefined);
        if (normalized) {
          const filtered = this.omitPreviouslyDisplayedUnlessReferenced(
            normalized,
            taskEvent.final_answer,
            displayedSnapshot,
          );
          taskEvent.attachments = filtered;
          if (filtered) {
            this.upsertMany(filtered, eventTimestamp);
          }
          break;
        }

        const fallback = this.hydrateFromContent(taskEvent.final_answer, {
          skipDisplayed: true,
          timestamp: eventTimestamp,
        });
        if (fallback) {
          const filtered = this.omitPreviouslyDisplayedUnlessReferenced(
            fallback,
            taskEvent.final_answer,
            displayedSnapshot,
          );
          taskEvent.attachments = filtered;
          if (filtered) {
            this.upsertMany(filtered, eventTimestamp);
          }
          break;
        }

        const rendered = this.hydrateFromContent(taskEvent.final_answer, {
          timestamp: eventTimestamp,
        });
        if (rendered) {
          taskEvent.attachments = this.omitPreviouslyDisplayedUnlessReferenced(
            rendered,
            taskEvent.final_answer,
            displayedSnapshot,
          );
        }
        if (!taskEvent.attachments) {
          const allowDisplayed = shouldIncludeDisplayedAttachments(taskEvent);
          const fromStore = this.takeFromStore({
            includeDisplayed: allowDisplayed,
            includeRecalled: false,
            timestamp: eventTimestamp,
          });
          taskEvent.attachments = this.omitPreviouslyDisplayedUnlessReferenced(
            fromStore,
            taskEvent.final_answer,
            displayedSnapshot,
          );
        }
        break;
      }
      default:
        break;
    }
  }
}

function normalizeAttachmentContent(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (value == null) {
    return "";
  }
  try {
    return JSON.stringify(value);
  } catch {
    return String(value);
  }
}

const attachmentRegistry = new AttachmentRegistry();

export const handleAttachmentEvent = (event: AnyAgentEvent) => {
  attachmentRegistry.handleEvent(event);
};

export const resetAttachmentRegistry = () => {
  attachmentRegistry.clear();
};

export const ingestRecalledAttachments = (
  attachments?: AttachmentMap | null,
  timestamp?: number,
) => {
  if (!attachments || Object.keys(attachments).length === 0) return;
  attachmentRegistry.ingestRecalled(attachments, timestamp);
};
