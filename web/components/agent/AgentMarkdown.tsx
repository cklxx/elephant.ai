"use client";

import { Children, useMemo } from "react";
import type { ComponentType, ReactNode } from "react";

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

function renderLineBreaks(children: ReactNode) {
  const nodes = Children.toArray(children);
  const output: ReactNode[] = [];
  nodes.forEach((child, childIndex) => {
    if (typeof child !== "string") {
      output.push(child);
      return;
    }
    const parts = child.split("\n");
    parts.forEach((part, partIndex) => {
      if (partIndex > 0) {
        output.push(<br key={`br-${childIndex}-${partIndex}`} />);
      }
      if (part !== "") {
        output.push(part);
      }
    });
  });
  return output;
}

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
        className="block font-mono text-xs leading-snug text-foreground"
        {...props}
      >
        {children}
      </code>
    );
  },
  pre: ({ children }: any) => (
    <pre className="markdown-code-block relative my-2 overflow-x-auto rounded-md border border-border/60 bg-muted/20 p-4">
      {children}
    </pre>
  ),
  p: ({ children }: any) => (
    <div className="whitespace-pre-wrap leading-snug text-foreground">
      {renderLineBreaks(children)}
    </div>
  ),
  ul: ({ children }: any) => children,
  ol: ({ children }: any) => children,
  li: ({ children }: any) => renderLineBreaks(children),
  strong: ({ children }: any) => (
    <strong className="font-medium text-foreground">{children}</strong>
  ),
  br: () => null,
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
