import type { AttachmentPayload } from "@/lib/types";
import { buildAttachmentUri, getAttachmentSegmentType } from "@/lib/attachments";

interface InlineAttachmentEntry {
  key: string;
  type: string;
  description?: string;
  mime?: string;
  attachment: AttachmentPayload;
}

interface UseInlineAttachmentMapOptions {
  content: string;
  attachments?: Record<string, AttachmentPayload>;
}

export function useInlineAttachmentMap({
  content,
  attachments,
}: UseInlineAttachmentMapOptions) {
  // Parse inline images from markdown content
  const inlineImageMap = new Map<string, string>();
  const imageRegex = /!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;
  let match: RegExpExecArray | null;
  while ((match = imageRegex.exec(content)) !== null) {
    const alt = (match[1] || "").trim();
    const url = (match[2] || "").trim();
    if (!url) {
      continue;
    }
    const key = alt || url;
    if (!inlineImageMap.has(key)) {
      inlineImageMap.set(key, url);
    }
  }

  // Build attachment map
  if (!attachments) {
    return {
      inlineAttachmentMap: new Map<string, InlineAttachmentEntry>(),
      attachmentNames: [] as string[],
      hasAttachments: false,
      inlineImageMap,
    };
  }

  const inlineAttachmentMap = new Map<string, InlineAttachmentEntry>();
  const attachmentNames: string[] = [];

  for (const [key, attachment] of Object.entries(attachments)) {
    const uri = buildAttachmentUri(attachment);
    if (uri) {
      inlineAttachmentMap.set(uri, {
        key,
        type: getAttachmentSegmentType(attachment),
        description: attachment.description,
        mime: attachment.media_type,
        attachment,
      });
    }
    const label = attachment.description || attachment.name || key;
    if (label) {
      attachmentNames.push(label);
    }
  }

  return {
    inlineAttachmentMap,
    attachmentNames,
    hasAttachments: attachmentNames.length > 0,
    inlineImageMap,
  };
}
