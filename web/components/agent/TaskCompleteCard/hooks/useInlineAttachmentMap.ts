import { useMemo } from "react";

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
  const inlineImageMap = useMemo(() => {
    const map = new Map<string, string>();
    const imageRegex = /!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;
    let match: RegExpExecArray | null;
    while ((match = imageRegex.exec(content)) !== null) {
      const alt = (match[1] || "").trim();
      const url = (match[2] || "").trim();
      if (!url) {
        continue;
      }
      const key = alt || url;
      if (!map.has(key)) {
        map.set(key, url);
      }
    }
    return map;
  }, [content]);

  const { inlineAttachmentMap, attachmentNames, hasAttachments } = useMemo(() => {
    if (!attachments) {
      return {
        inlineAttachmentMap: new Map<string, InlineAttachmentEntry>(),
        attachmentNames: [] as string[],
        hasAttachments: false,
      };
    }

    const inlineMap = new Map<string, InlineAttachmentEntry>();
    const names: string[] = [];

    Object.entries(attachments).forEach(([key, attachment]) => {
      const uri = buildAttachmentUri(attachment);
      if (uri) {
        inlineMap.set(uri, {
          key,
          type: getAttachmentSegmentType(attachment),
          description: attachment.description,
          mime: attachment.media_type,
          attachment,
        });
      }
      const label = attachment.description || attachment.name || key;
      if (label) {
        names.push(label);
      }
    });

    return {
      inlineAttachmentMap: inlineMap,
      attachmentNames: names,
      hasAttachments: names.length > 0,
    };
  }, [attachments]);

  return { inlineAttachmentMap, attachmentNames, hasAttachments, inlineImageMap };
}
