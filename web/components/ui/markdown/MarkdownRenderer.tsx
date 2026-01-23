"use client";

import { Streamdown } from "streamdown";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import rehypeKatex from "rehype-katex";
import { harden } from "rehype-harden";
import remarkGfm from "remark-gfm";
import { useMemo } from "react";
import { cn } from "@/lib/utils";
import { buildAttachmentUri, getAttachmentSegmentType } from "@/lib/attachments";
import type { AttachmentPayload } from "@/lib/types";

import { useMarkdownComponents } from "./hooks/useMarkdownComponents";

const SANITIZE_SCHEMA = {
  ...defaultSchema,
  tagNames: [
    ...(defaultSchema.tagNames ?? []),
    "table",
    "thead",
    "tbody",
    "tr",
    "th",
    "td",
    "caption",
    "colgroup",
    "col",
  ],
  attributes: {
    ...defaultSchema.attributes,
    table: [...(defaultSchema.attributes?.table ?? []), "className"],
    th: [...(defaultSchema.attributes?.th ?? []), "align"],
    td: [...(defaultSchema.attributes?.td ?? []), "align"],
  },
};

const REHYPE_PLUGINS = [
  rehypeRaw,
  [rehypeSanitize, SANITIZE_SCHEMA],
  [rehypeKatex, { errorColor: "var(--color-muted-foreground)" }],
  [
    harden,
    {
      allowedImagePrefixes: ["*"],
      allowedLinkPrefixes: ["*"],
      allowedProtocols: ["*"],
      allowDataImages: true,
    },
  ],
];

const REMARK_PLUGINS = [remarkGfm];

export type MarkdownRendererProps = {
  content: string;
  containerClassName?: string;
  className?: string;
  showLineNumbers?: boolean;
  mode?: "static" | "streaming";
  components?: Record<string, React.ComponentType<any>>;
  attachments?: Record<string, AttachmentPayload>;
};

export function MarkdownRenderer({
  content,
  containerClassName,
  className,
  showLineNumbers = false,
  mode = "static",
  components,
  attachments,
}: MarkdownRendererProps) {
  const inlineAttachmentMap = useMemo(() => {
    if (!attachments) {
      return new Map<
        string,
        {
          type: string;
          description?: string;
          mime?: string;
        }
      >();
    }

    return Object.values(attachments).reduce(
      (acc, attachment) => {
        const uri = buildAttachmentUri(attachment);
        if (!uri) {
          return acc;
        }
        acc.set(uri, {
          type: getAttachmentSegmentType(attachment),
          description: attachment.description,
          mime: attachment.media_type,
        });
        return acc;
      },
      new Map<
        string,
        {
          type: string;
          description?: string;
          mime?: string;
        }
      >(),
    );
  }, [attachments]);

  const mergedComponents = useMarkdownComponents({
    showLineNumbers,
    inlineAttachmentMap,
    components,
  });

  return (
    <div className={cn("markdown-body", containerClassName)}>
      <Streamdown
        mode={mode}
        className={cn(
          "wmde-markdown markdown-body__content prose prose-sm max-w-none text-foreground prose-a:break-words prose-a:whitespace-normal",
          className,
        )}
        components={mergedComponents as any}
        remarkPlugins={REMARK_PLUGINS as any}
        rehypePlugins={REHYPE_PLUGINS as any}
      >
        {content}
      </Streamdown>
    </div>
  );
}
