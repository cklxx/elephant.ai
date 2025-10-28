"use client";

import "@uiw/react-markdown-preview/dist/markdown.css";

import MarkdownPreview from "@uiw/react-markdown-preview";
import remarkGfm from "remark-gfm";
import { Highlight, Language, themes } from "prism-react-renderer";
import { cn } from "@/lib/utils";
import { ComponentType } from "react";

type MarkdownRendererProps = {
  content: string;
  className?: string;
  showLineNumbers?: boolean;
  components?: Record<string, ComponentType<any>>;
};

export function MarkdownRenderer({
  content,
  className,
  showLineNumbers = false,
  components,
}: MarkdownRendererProps) {
  const defaultCodeRenderer = ({ className, children, ...props }: any) => {
    const match = /language-(\w+)/.exec(className || "");
    const language = (match?.[1] as Language | undefined) ?? "text";
    const isInline = !match;

    if (isInline) {
      return (
        <code
          className="bg-muted text-foreground px-1.5 py-0.5 rounded font-mono text-[0.9em] whitespace-nowrap"
          {...props}
        >
          {children}
        </code>
      );
    }

    return (
      <Highlight theme={themes.vsDark} code={String(children).replace(/\n$/, "")} language={language}>
        {({ className: resolvedClassName, style, tokens, getLineProps, getTokenProps }) => (
          <pre className={cn(resolvedClassName, "rounded-lg overflow-auto border border-border bg-gray-950/95")}
               style={style}
               {...props}
          >
            {tokens.map((line, index) => (
              <div key={index} {...getLineProps({ line })}>
                {showLineNumbers && (
                  <span className="inline-block w-8 select-none text-right pr-3 text-xs text-gray-500">
                    {index + 1}
                  </span>
                )}
                {line.map((token, key) => (
                  <span key={key} {...getTokenProps({ token })} />
                ))}
              </div>
            ))}
          </pre>
        )}
      </Highlight>
    );
  };

  const mergedComponents = {
    ...components,
  };

  if (!components?.code) {
    mergedComponents.code = defaultCodeRenderer;
  }

  return (
    <MarkdownPreview
      className={cn("prose prose-sm max-w-none text-foreground", className)}
      source={content}
      remarkPlugins={[remarkGfm]}
      wrapperElement={{ "data-color-mode": "light" }}
      components={mergedComponents as any}
    />
  );
}

