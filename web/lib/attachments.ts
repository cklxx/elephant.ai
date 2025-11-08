import { AttachmentPayload } from '@/lib/types';

const PLACEHOLDER_REGEX = /\[([^\[\]]+)\]/g;

export interface ContentSegment {
  type: 'text' | 'image';
  text?: string;
  placeholder?: string;
  attachment?: AttachmentPayload;
}

export function buildAttachmentUri(
  attachment: AttachmentPayload,
): string | null {
  const direct = attachment.uri?.trim();
  if (direct) {
    return direct;
  }
  const data = attachment.data?.trim();
  if (!data) {
    return null;
  }
  const mediaType = attachment.media_type?.trim() || 'application/octet-stream';
  return `data:${mediaType};base64,${data}`;
}

export function replacePlaceholdersWithMarkdown(
  content: string,
  attachments?: Record<string, AttachmentPayload>,
): string {
  if (!content || !attachments || Object.keys(attachments).length === 0) {
    return content;
  }
  return content.replace(PLACEHOLDER_REGEX, (match, rawName) => {
    const name = String(rawName).trim();
    if (!name) {
      return match;
    }
    const attachment = attachments[name];
    if (!attachment) {
      return match;
    }
    const uri = buildAttachmentUri(attachment);
    if (!uri) {
      return match;
    }
    const alt = attachment.description || name;
    return `![${alt}](${uri})`;
  });
}

export function parseContentSegments(
  content: string,
  attachments?: Record<string, AttachmentPayload>,
): ContentSegment[] {
  const normalizedContent = typeof content === 'string' ? content : '';
  const attachmentEntries = attachments ? Object.entries(attachments) : [];
  const segments: ContentSegment[] = [];
  const usedAttachments = new Set<string>();

  if (normalizedContent.length > 0) {
    let lastIndex = 0;
    let match: RegExpExecArray | null;

    while ((match = PLACEHOLDER_REGEX.exec(normalizedContent)) !== null) {
      const start = match.index;
      const end = start + match[0].length;
      const name = String(match[1]).trim();

      if (start > lastIndex) {
        segments.push({
          type: 'text',
          text: normalizedContent.slice(lastIndex, start),
        });
      }

      const attachment = attachments?.[name];
      if (attachment) {
        usedAttachments.add(name);
        segments.push({
          type: 'image',
          placeholder: match[0],
          attachment: {
            ...attachment,
            name: attachment.name || name,
          },
        });
      } else {
        segments.push({ type: 'text', text: match[0] });
      }

      lastIndex = end;
    }

    if (lastIndex < normalizedContent.length) {
      segments.push({
        type: 'text',
        text: normalizedContent.slice(lastIndex),
      });
    }

    if (segments.length === 0) {
      segments.push({ type: 'text', text: normalizedContent });
    }
  } else if (attachmentEntries.length === 0) {
    return [{ type: 'text', text: '' }];
  }

  if (attachmentEntries.length > 0) {
    attachmentEntries.forEach(([key, attachment]) => {
      const normalizedKey = key.trim();
      if (!normalizedKey || usedAttachments.has(normalizedKey)) {
        return;
      }
      segments.push({
        type: 'image',
        placeholder: `[${normalizedKey}]`,
        attachment: {
          ...attachment,
          name: attachment.name || normalizedKey,
        },
      });
    });
  }

  return segments.length > 0 ? segments : [{ type: 'text', text: normalizedContent }];
}
