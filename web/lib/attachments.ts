import { AttachmentPayload } from '@/lib/types';

const PLACEHOLDER_REGEX = /\[([^\[\]]+)\]/g;
const IMAGE_MARKDOWN_REGEX = /!\[([^\]]*)\]\(([^)\s]+)(?:\s+"([^"]*)")?\)/g;

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

  const normalizedAttachments = attachmentEntries.reduce<Record<string, AttachmentPayload>>((acc, [key, attachment]) => {
    const normalizedKey = key.trim();
    if (!normalizedKey) {
      return acc;
    }
    acc[normalizedKey] = {
      ...attachment,
      name: attachment.name?.trim() || normalizedKey,
    };
    return acc;
  }, {});

  const attachmentList = Object.entries(normalizedAttachments).map(([key, attachment]) => ({
    key,
    attachment,
    uri: buildAttachmentUri(attachment),
    description: attachment.description?.trim(),
    name: attachment.name?.trim() || key,
  }));

  if (normalizedContent.length > 0) {
    const tokens = tokenizeContent(normalizedContent);
    tokens.forEach((token) => {
      if (token.type === 'markdownImage') {
        const matchedKey = findAttachmentKeyFromMarkdown(token, attachmentList);
        if (matchedKey && normalizedAttachments[matchedKey]) {
          usedAttachments.add(matchedKey);
          segments.push({
            type: 'image',
            placeholder: `[${matchedKey}]`,
            attachment: normalizedAttachments[matchedKey],
          });
          return;
        }
        segments.push({ type: 'text', text: token.raw });
        return;
      }

      const textSegments = extractPlaceholderSegments(
        token.value,
        normalizedAttachments,
        usedAttachments,
      );
      if (textSegments.length === 0) {
        segments.push({ type: 'text', text: token.value });
      } else {
        segments.push(...textSegments);
      }
    });

    if (segments.length === 0) {
      segments.push({ type: 'text', text: normalizedContent });
    }
  } else if (attachmentEntries.length === 0) {
    return [{ type: 'text', text: '' }];
  }

  if (attachmentList.length > 0) {
    attachmentList.forEach(({ key, attachment }) => {
      if (usedAttachments.has(key)) {
        return;
      }
      segments.push({
        type: 'image',
        placeholder: `[${key}]`,
        attachment,
      });
    });
  }

  return segments.length > 0 ? segments : [{ type: 'text', text: normalizedContent }];
}

type ContentToken =
  | { type: 'text'; value: string }
  | { type: 'markdownImage'; raw: string; alt?: string; url?: string; title?: string };

function tokenizeContent(content: string): ContentToken[] {
  const tokens: ContentToken[] = [];
  const regex = new RegExp(IMAGE_MARKDOWN_REGEX);
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(content)) !== null) {
    const start = match.index;
    const end = start + match[0].length;
    if (start > lastIndex) {
      tokens.push({
        type: 'text',
        value: content.slice(lastIndex, start),
      });
    }
    tokens.push({
      type: 'markdownImage',
      raw: match[0],
      alt: match[1],
      url: match[2],
      title: match[3],
    });
    lastIndex = end;
  }

  if (lastIndex < content.length) {
    tokens.push({
      type: 'text',
      value: content.slice(lastIndex),
    });
  }

  if (tokens.length === 0) {
    return [{ type: 'text', value: content }];
  }

  return tokens;
}

function extractPlaceholderSegments(
  text: string,
  attachments: Record<string, AttachmentPayload>,
  usedAttachments: Set<string>,
): ContentSegment[] {
  if (!text) {
    return [];
  }

  const segments: ContentSegment[] = [];
  const regex = new RegExp(PLACEHOLDER_REGEX);
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(text)) !== null) {
    const start = match.index;
    const end = start + match[0].length;
    const name = String(match[1]).trim();

    if (start > lastIndex) {
      segments.push({
        type: 'text',
        text: text.slice(lastIndex, start),
      });
    }

    const attachment = attachments[name];
    if (attachment) {
      usedAttachments.add(name);
      segments.push({
        type: 'image',
        placeholder: match[0],
        attachment,
      });
    } else {
      segments.push({ type: 'text', text: match[0] });
    }

    lastIndex = end;
  }

  if (lastIndex < text.length) {
    segments.push({
      type: 'text',
      text: text.slice(lastIndex),
    });
  }

  return segments;
}

interface AttachmentDetail {
  key: string;
  attachment: AttachmentPayload;
  uri: string | null;
  description?: string;
  name?: string;
}

function findAttachmentKeyFromMarkdown(
  token: Extract<ContentToken, { type: 'markdownImage' }>,
  attachments: AttachmentDetail[],
): string | undefined {
  if (attachments.length === 0) {
    return undefined;
  }

  const normalizedTitle = token.title?.trim();
  if (normalizedTitle) {
    const byTitle = attachments.find((item) => item.key === normalizedTitle);
    if (byTitle) {
      return byTitle.key;
    }
  }

  const normalizedUrl = token.url?.trim();
  if (normalizedUrl) {
    const byUrl = attachments.find((item) => item.uri && item.uri === normalizedUrl);
    if (byUrl) {
      return byUrl.key;
    }
  }

  const normalizedAlt = token.alt?.trim();
  if (normalizedAlt) {
    const byAlt = attachments.find(
      (item) =>
        item.key === normalizedAlt ||
        item.name === normalizedAlt ||
        item.description === normalizedAlt,
    );
    if (byAlt) {
      return byAlt.key;
    }
  }

  return undefined;
}
