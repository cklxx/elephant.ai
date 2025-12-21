import { buildApiUrl } from '@/lib/api-base';
import { AttachmentPayload } from '@/lib/types';

const PLACEHOLDER_REGEX = /\[([^\[\]]+)\]/g;
const IMAGE_MARKDOWN_REGEX = /!\[([^\]]*)\]\(([^)\s]+)(?:\s+"([^"]*)")?\)/g;

export interface ContentSegment {
  type: 'text' | 'image' | 'video' | 'document' | 'embed';
  text?: string;
  placeholder?: string;
  attachment?: AttachmentPayload;
  implicit?: boolean;
}

export type AttachmentSegmentType = ContentSegment['type'];

export function buildAttachmentUri(
  attachment: AttachmentPayload,
): string | null {
  const direct = attachment.uri?.trim();
  if (direct) {
    return normalizeAttachmentUri(direct);
  }
  const data = attachment.data?.trim();
  if (!data) {
    const previewAssets = attachment.preview_assets ?? [];
    const preferredAsset = previewAssets.find((asset) => {
      const mime = asset.mime_type?.toLowerCase() ?? '';
      const previewType = asset.preview_type?.toLowerCase() ?? '';
      return (
        Boolean(asset.cdn_url?.trim()) &&
        (mime.startsWith('video/') || previewType.includes('video'))
      );
    });
    const fallbackAsset = previewAssets.find((asset) => asset.cdn_url?.trim());
    const cdnUrl = preferredAsset?.cdn_url?.trim() || fallbackAsset?.cdn_url?.trim();
    return cdnUrl ? normalizeAttachmentUri(cdnUrl) : null;
  }
  // If the payload already contains a data URI or a full URL, return it directly.
  if (
    data.startsWith('data:') ||
    data.startsWith('http://') ||
    data.startsWith('https://') ||
    data.startsWith('/')
  ) {
    return normalizeAttachmentUri(data);
  }
  const mediaType = attachment.media_type?.trim() || 'application/octet-stream';
  return `data:${mediaType};base64,${data}`;
}

function normalizeAttachmentUri(uri: string): string {
  const trimmed = uri.trim();
  if (
    trimmed.startsWith('data:') ||
    trimmed.startsWith('http://') ||
    trimmed.startsWith('https://')
  ) {
    return trimmed;
  }
  if (trimmed.startsWith('/')) {
    return buildApiUrl(trimmed);
  }
  return trimmed;
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
    type: getAttachmentSegmentType(attachment),
  }));

  const attachmentTypes = attachmentList.reduce<Record<string, AttachmentSegmentType>>(
    (acc, { key, type }) => {
      acc[key] = type;
      return acc;
    },
    {},
  );

  if (normalizedContent.length > 0) {
    const tokens = tokenizeContent(normalizedContent);
    tokens.forEach((token) => {
      if (token.type === 'markdownImage') {
        const matchedKey = findAttachmentKeyFromMarkdown(token, attachmentList);
        if (matchedKey && normalizedAttachments[matchedKey]) {
          usedAttachments.add(matchedKey);
          segments.push({
            type: attachmentTypes[matchedKey] ?? 'image',
            placeholder: `[${matchedKey}]`,
            attachment: normalizedAttachments[matchedKey],
          });
          return;
        }
        if (token.url) {
          const syntheticAttachment: AttachmentPayload = {
            name: token.alt || token.url,
            description: token.title || token.alt,
            uri: token.url,
            media_type: inferMediaTypeFromUrl(token.url),
          };
          segments.push({
            type: 'image',
            placeholder: token.raw,
            attachment: syntheticAttachment,
          });
        } else {
          segments.push({ type: 'text', text: token.raw });
        }
        return;
      }

      const textSegments = extractPlaceholderSegments(
        token.value,
        normalizedAttachments,
        attachmentTypes,
        usedAttachments,
      );
      const expanded = (textSegments.length === 0
        ? [{ type: 'text', text: token.value } as ContentSegment]
        : textSegments
      ).flatMap((segment) => {
        if (segment.type === 'text' && segment.text) {
          const split = splitInlineMedia(segment.text);
          if (split.length > 0) {
            return split;
          }
        }
        return segment;
      });
      segments.push(...expanded);
    });

    if (segments.length === 0) {
      segments.push({ type: 'text', text: normalizedContent });
    }
  } else if (attachmentEntries.length === 0) {
    return [{ type: 'text', text: '' }];
  }

  // Append unused attachments as implicit segments
  if (attachments && Object.keys(attachments).length > 0) {
    Object.entries(attachments).forEach(([key, attachment]) => {
      // If it was used in text content, skip it
      if (usedAttachments.has(key)) {
        return;
      }

      // Determine type based on existing logic or default to image
      const type = attachmentTypes[key] ?? 'image';

      segments.push({
        type,
        placeholder: `[${key}]`,
        attachment,
        implicit: true,
      });
    });
  }

  return segments.length > 0 ? segments : [{ type: 'text', text: normalizedContent }];
}

type ContentToken =
  | { type: 'text'; value: string }
  | { type: 'markdownImage'; raw: string; alt?: string; url?: string; title?: string };

function inferMediaTypeFromUrl(url: string | undefined): string {
  if (!url) return 'application/octet-stream';
  const trimmed = url.trim();
  if (trimmed.startsWith('data:')) {
    const header = trimmed.slice(5).split(';')[0];
    return header || 'application/octet-stream';
  }
  const extMatch = trimmed.match(/\.([a-zA-Z0-9]+)(?:\?|#|$)/);
  if (extMatch?.[1]) {
    const ext = extMatch[1].toLowerCase();
    switch (ext) {
      case 'png':
        return 'image/png';
      case 'jpg':
      case 'jpeg':
        return 'image/jpeg';
      case 'gif':
        return 'image/gif';
      case 'webp':
        return 'image/webp';
      case 'svg':
        return 'image/svg+xml';
      default:
        return `image/${ext}`;
    }
  }
  return 'application/octet-stream';
}

function splitInlineMedia(text: string): ContentSegment[] {
  if (!text) return [];
  const segments: ContentSegment[] = [];
  const regex = /(data:[^\s)]+base64,[A-Za-z0-9+/=]+|\/api\/data\/[A-Za-z0-9]+)/g;
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(text)) !== null) {
    const start = match.index;
    const end = start + match[0].length;
    if (start > lastIndex) {
      segments.push({ type: 'text', text: text.slice(lastIndex, start) });
    }
    const url = match[0];
    segments.push({
      type: 'image',
      placeholder: url,
      attachment: {
        name: url,
        uri: url,
        media_type: inferMediaTypeFromUrl(url),
      },
    });
    lastIndex = end;
  }

  if (lastIndex < text.length) {
    segments.push({ type: 'text', text: text.slice(lastIndex) });
  }

  return segments;
}

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
  attachmentTypes: Record<string, AttachmentSegmentType>,
  usedAttachments: Set<string>,
): ContentSegment[] {
  if (!text) {
    return [];
  }

  const segments: ContentSegment[] = [];
  let remainingText = text;

  // 1. Suffix Match at Start (Fix for truncated start like "ame.png]")
  // Matches start of string, capturing chars until ']' (excluding brackets to avoid internal matches)
  const startSuffixRegex = /^([^\[\]]+)\]/;
  const startMatch = startSuffixRegex.exec(remainingText);
  if (startMatch) {
    const suffix = startMatch[1].trim();
    if (suffix) {
      // Find key that ends with this suffix
      const matchedKey = Object.keys(attachments).find((key) => key.endsWith(suffix));
      if (matchedKey) {
        usedAttachments.add(matchedKey);
        segments.push({
          type: attachmentTypes[matchedKey] ?? 'image',
          placeholder: `[${matchedKey}]`,
          attachment: attachments[matchedKey],
        });
        remainingText = remainingText.slice(startMatch[0].length);
      }
    }
  }

  // 2. Prefix Match at End (Fix for truncated end like "[fil")
  // Matches last '[' followed by non-bracket chars to end of string
  const endPrefixRegex = /\[([^\[\]]+)$/;
  const endMatch = endPrefixRegex.exec(remainingText);
  let endSegment: ContentSegment | null = null;

  if (endMatch) {
    const prefix = endMatch[1].trim();
    if (prefix) {
      const matchedKey = Object.keys(attachments).find((key) => key.startsWith(prefix));
      if (matchedKey) {
        usedAttachments.add(matchedKey);
        endSegment = {
          type: attachmentTypes[matchedKey] ?? 'image',
          placeholder: `[${matchedKey}]`,
          attachment: attachments[matchedKey],
        };
        // Slice off the prefix part
        remainingText = remainingText.slice(0, endMatch.index);
      }
    }
  }

  // 3. Standard parsing for the middle
  const regex = new RegExp(PLACEHOLDER_REGEX);
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = regex.exec(remainingText)) !== null) {
    const start = match.index;
    const end = start + match[0].length;
    const name = String(match[1]).trim();

    if (start > lastIndex) {
      segments.push({
        type: 'text',
        text: remainingText.slice(lastIndex, start),
      });
    }

    const attachment = attachments[name];
    if (attachment) {
      usedAttachments.add(name);
      segments.push({
        type: attachmentTypes[name] ?? 'image',
        placeholder: match[0],
        attachment,
      });
    } else {
      segments.push({ type: 'text', text: match[0] });
    }

    lastIndex = end;
  }

  if (lastIndex < remainingText.length) {
    segments.push({
      type: 'text',
      text: remainingText.slice(lastIndex),
    });
  }

  // 4. Append the end segment if we found one
  if (endSegment) {
    segments.push(endSegment);
  }

  return segments;
}

const DOCUMENT_FORMATS = new Set(['ppt', 'pptx', 'pdf', 'markdown', 'md', 'doc', 'docx']);

export function getAttachmentSegmentType(
  attachment: AttachmentPayload,
): AttachmentSegmentType {
  const mediaType = attachment.media_type?.toLowerCase() ?? '';
  const mediaSubtype =
    mediaType.includes('/') && mediaType.split('/').length > 1
      ? mediaType.split('/')[1]?.split(';')[0]
      : '';
  if (mediaType.startsWith('video/')) {
    return 'video';
  }

  const format = attachment.format?.toLowerCase();
  const normalizedFormat = format || mediaSubtype;
  const previewProfile = attachment.preview_profile?.toLowerCase() ?? '';
  const kind = attachment.kind?.toLowerCase();
  const previewAssets = attachment.preview_assets ?? [];
  const hasPreviewAssets = previewAssets.length > 0;
  const hasVideoAsset = previewAssets.some((asset) => {
    const mime = asset.mime_type?.toLowerCase() ?? '';
    const previewType = asset.preview_type?.toLowerCase() ?? '';
    return mime.startsWith('video/') || previewType.includes('video');
  });
  if (hasVideoAsset) {
    return 'video';
  }
  const hasHtmlAsset = previewAssets.some((asset) => {
    const mime = asset.mime_type?.toLowerCase() ?? '';
    const previewType = asset.preview_type?.toLowerCase() ?? '';
    return mime.includes('html') || previewType.includes('html') || previewType.includes('iframe');
  });
  const htmlLike =
    mediaType === 'text/html' ||
    normalizedFormat === 'html' ||
    previewProfile.includes('html') ||
    hasHtmlAsset;
  if (htmlLike) {
    return 'embed';
  }

  const documentProfile = previewProfile.startsWith('document.');
  const markdownFormat =
    normalizedFormat === 'markdown' || normalizedFormat === 'md' || normalizedFormat === 'x-markdown';
  const documentFormat =
    (normalizedFormat && DOCUMENT_FORMATS.has(normalizedFormat)) || normalizedFormat === 'pdf' || markdownFormat;
  const isArtifact = kind === 'artifact';
  if (
    isArtifact ||
    documentProfile ||
    documentFormat ||
    hasPreviewAssets ||
    (mediaType.startsWith('text/') && !htmlLike)
  ) {
    return 'document';
  }

  return 'image';
}

export function isA2UIAttachment(attachment: AttachmentPayload): boolean {
  const mediaType = attachment.media_type?.toLowerCase() ?? '';
  const format = attachment.format?.toLowerCase() ?? '';
  const previewProfile = attachment.preview_profile?.toLowerCase() ?? '';
  return mediaType.includes('a2ui') || format === 'a2ui' || previewProfile.includes('a2ui');
}

interface AttachmentDetail {
  key: string;
  attachment: AttachmentPayload;
  uri: string | null;
  description?: string;
  name?: string;
  type: ContentSegment['type'];
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
