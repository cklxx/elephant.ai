"use client";

import { Children, isValidElement, useMemo } from "react";
import type { ComponentType, ReactNode } from "react";

import type { AttachmentPayload } from "@/lib/types";
import { cn } from "@/lib/utils";
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

function splitTaskListChildren(children: ReactNode) {
  const nodes = Children.toArray(children);
  const checkboxIndex = nodes.findIndex(
    (child) =>
      isValidElement(child) &&
      child.type === "input" &&
      (child.props as { type?: string }).type === "checkbox",
  );
  if (checkboxIndex === -1) {
    return null;
  }
  const checkbox = nodes[checkboxIndex];
  const rest = nodes.filter((_, index) => index !== checkboxIndex);
  return { checkbox, rest };
}

const baseComponents: Record<string, ComponentType<any>> = {
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
  ul: ({ className, ...props }: any) => {
    const isTaskList =
      typeof className === "string" && className.includes("contains-task-list");
    return (
      <ul
        className={cn(
          "my-2 !pl-0 flex flex-col gap-1",
          isTaskList ? "list-none" : "list-disc list-inside",
          className,
        )}
        {...props}
      />
    );
  },
  ol: ({ className, ...props }: any) => {
    const isTaskList =
      typeof className === "string" && className.includes("contains-task-list");
    return (
      <ol
        className={cn(
          "my-2 !pl-0 flex flex-col gap-1",
          isTaskList ? "list-none" : "list-decimal list-inside",
          className,
        )}
        {...props}
      />
    );
  },
  li: ({ className, children, ...props }: any) => {
    const taskChildren = splitTaskListChildren(children);
    if (taskChildren) {
      return (
        <li className={cn("flex items-start gap-2", className)} {...props}>
          {taskChildren.checkbox}
          <div className="min-w-0 flex-1">
            {renderLineBreaks(taskChildren.rest)}
          </div>
        </li>
      );
    }
    return (
      <li className={cn("", className)} {...props}>
        {renderLineBreaks(children)}
      </li>
    );
  },
  strong: ({ children }: any) => (
    <strong className="font-medium text-foreground">{children}</strong>
  ),
  br: () => null,
  input: ({ className, type, ...props }: any) => {
    if (type === "checkbox") {
      return (
        <input
          type="checkbox"
          className={cn("mt-1 h-4 w-4 shrink-0 accent-foreground", className)}
          {...props}
        />
      );
    }
    return <input type={type} className={className} {...props} />;
  },
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
