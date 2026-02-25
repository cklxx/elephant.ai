import { Highlight, Language, themes } from "prism-react-renderer";
import type { ComponentType } from "react";

import { cn } from "@/lib/utils";

interface MarkdownCodeProps {
  className?: string;
  children?: string | string[];
  showLineNumbers?: boolean;
}

export function createMarkdownCodeRenderer(
  showLineNumbers: boolean,
): ComponentType<MarkdownCodeProps> {
  return function MarkdownCode({ className, children, ...props }: MarkdownCodeProps) {
    const match = /language-(\w+)/.exec(className || "");
    const language = (match?.[1] as Language | undefined) ?? "text";
    const isInline = !match;

    if (isInline) {
      return (
        <code className={cn(className)} {...props}>
          {children}
        </code>
      );
    }

    return (
      <Highlight
        theme={themes.vsDark}
        code={String(children).replace(/\n$/, "")}
        language={language}
      >
        {({
          className: resolvedClassName,
          style,
          tokens,
          getLineProps,
          getTokenProps,
        }) => (
          <pre
            className={cn(resolvedClassName, className)}
            style={style}
            {...props}
          >
            {tokens.map((line, index) => (
              <div key={index} {...getLineProps({ line })}>
                {showLineNumbers && (
                  <span className="inline-block w-10 select-none text-right pr-4 text-xs text-gray-500">
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
}
