"use client";

import { useMemo } from "react";
import type { ComponentType } from "react";

import type { AttachmentPayload } from "@/lib/types";
import { StreamingMarkdownRenderer } from "./StreamingMarkdownRenderer";

export type AgentMarkdownProps = {
  content: string;
  className?: string;
  containerClassName?: string;
  attachments?: Record<string, AttachmentPayload>;
  isStreaming?: boolean;
  streamFinished?: boolean;
  components?: Record<string, ComponentType<any>>;
  showLineNumbers?: boolean;
};

const baseComponents: Record<string, ComponentType<any>> = {
  code: ({ inline, children, ...props }: any) => {
    if (inline) {
      return (
        <code
          className="whitespace-nowrap rounded bg-muted px-1.5 py-0.5 font-mono text-xs text-foreground"
          {...props}
        >
          {children}
        </code>
      );
    }
    return (
      <code
        className="block font-mono text-xs leading-relaxed text-foreground"
        {...props}
      >
        {children}
      </code>
    );
  },
  pre: ({ children }: any) => (
    <pre className="markdown-code-block relative my-4 overflow-x-auto rounded-md border border-border/60 bg-muted/20 p-4">
      {children}
    </pre>
  ),
  p: ({ children }: any) => (
    <div className="leading-relaxed text-foreground">{children}</div>
  ),
  ul: ({ children }: any) => (
    <ul className="mb-4 space-y-2 leading-relaxed text-foreground">
      {children}
    </ul>
  ),
  ol: ({ children }: any) => (
    <ol className="mb-4 space-y-2 leading-relaxed text-foreground">
      {children}
    </ol>
  ),
  li: ({ children }: any) => (
    <li className="leading-relaxed text-foreground">{children}</li>
  ),
  strong: ({ children }: any) => (
    <strong className="font-bold text-foreground">{children}</strong>
  ),
};

export function AgentMarkdown({
  content,
  className,
  containerClassName,
  attachments,
  isStreaming,
  streamFinished,
  components,
  showLineNumbers,
}: AgentMarkdownProps) {
  const mergedComponents = useMemo(
    () => ({
      ...baseComponents,
      ...(components ?? {}),
    }),
    [components],
  );

  return (
    <StreamingMarkdownRenderer
      content={content}
      className={className}
      containerClassName={containerClassName}
      components={mergedComponents}
      attachments={attachments}
      isStreaming={isStreaming}
      streamFinished={streamFinished}
      showLineNumbers={showLineNumbers}
    />
  );
}
