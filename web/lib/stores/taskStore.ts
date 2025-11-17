import { AttachmentPayload } from "@/lib/types";

export type AttachmentMap = Record<string, AttachmentPayload>;

const attachmentStore = new Map<string, AttachmentMap>();

function cloneMap(map?: AttachmentMap): AttachmentMap | undefined {
  if (!map) {
    return undefined;
  }
  return Object.fromEntries(
    Object.entries(map).map(([key, attachment]) => [key, { ...attachment }]),
  );
}

export function saveTaskAttachments(
  taskId: string | undefined,
  attachments: AttachmentMap | undefined,
) {
  if (!taskId || !attachments || Object.keys(attachments).length === 0) {
    return;
  }
  attachmentStore.set(taskId, cloneMap(attachments)!);
}

export function getTaskAttachments(
  taskId: string | undefined,
): AttachmentMap | undefined {
  if (!taskId) {
    return undefined;
  }
  return cloneMap(attachmentStore.get(taskId));
}

export function clearTaskAttachments(taskId?: string) {
  if (!taskId) {
    attachmentStore.clear();
    return;
  }
  attachmentStore.delete(taskId);
}
