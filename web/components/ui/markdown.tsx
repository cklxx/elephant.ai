"use client";
/* eslint-disable @next/next/no-img-element */

import MarkdownPreview from "@uiw/react-markdown-preview";
import remarkGfm from "remark-gfm";
import { Highlight, Language, themes } from "prism-react-renderer";
import { cn } from "@/lib/utils";
import { ComponentType, useMemo, useState } from "react";
import { VideoPreview } from "@/components/ui/video-preview";
import { buildAttachmentUri, getAttachmentSegmentType } from "@/lib/attachments";
import { AttachmentPayload } from "@/lib/types";
import { Dialog, DialogContent } from "@/components/ui/dialog";

type MarkdownRendererProps = {
  content: string;
  /**
   * Optional classes applied to the rendered markdown container. This is
   * typically where padding or layout adjustments should live.
   */
  containerClassName?: string;
  /**
   * Optional classes applied to the markdown content itself. Use this for
   * typography utilities such as `prose` modifiers.
   */
  className?: string;
  showLineNumbers?: boolean;
  components?: Record<string, ComponentType<any>>;
  attachments?: Record<string, AttachmentPayload>;
};

export function MarkdownRenderer({
  content,
  containerClassName,
  className,
  showLineNumbers = false,
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
      <Highlight
        theme={themes.vsDark}
        code={String(children).replace(/\n$/, "")}
        language={language}
      >
        {({ className: resolvedClassName, style, tokens, getLineProps, getTokenProps }) => (
          <pre
            className={cn(
              resolvedClassName,
              "markdown-code-block rounded-xl border border-border bg-gray-950/95 p-4 text-sm leading-relaxed text-gray-100 shadow-sm",
            )}
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

  const defaultComponents: Record<string, ComponentType<any>> = {
    hr: (props: any) => <hr className="my-6 border-border" {...props} />,
    a: ({ className: linkClassName, href, children, ...props }: any) => {
      const matchedAttachment = href ? inlineAttachmentMap.get(href) : undefined;

      if (matchedAttachment?.type === "video") {
        return (
          <VideoPreview
            src={href}
            mimeType={matchedAttachment.mime || "video/mp4"}
            description={matchedAttachment.description || (typeof children === "string" ? children : undefined)}
            className="my-2 max-w-full"
            maxHeight="360px"
          />
        );
      }

      return (
        <a
          className={cn(
            "text-primary underline decoration-2 underline-offset-4 transition-colors hover:text-primary/80",
            linkClassName,
          )}
          href={href}
          {...props}
        >
          {children}
        </a>
      );
    },
    blockquote: ({ className: blockquoteClass, ...props }: any) => (
      <blockquote
        className={cn(
          "border-l-4 border-primary/40 bg-primary/5 py-2 pl-4 pr-3 text-sm italic text-muted-foreground",
          blockquoteClass,
        )}
        {...props}
      />
    ),
    table: ({ className: tableClass, ...props }: any) => (
      <div className="my-4 overflow-x-auto rounded-xl border border-border">
        <table className={cn("w-full border-collapse text-sm", tableClass)} {...props} />
      </div>
    ),
    th: ({ className: thClass, ...props }: any) => (
      <th
        className={cn(
          "bg-muted/60 px-4 py-2 text-left text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground",
          thClass,
        )}
        {...props}
      />
    ),
    td: ({ className: tdClass, ...props }: any) => (
      <td className={cn("border-t border-border px-4 py-2 align-top text-sm", tdClass)} {...props} />
    ),
    ul: ({ className: ulClass, ...props }: any) => (
      <ul className={cn("my-4 list-disc space-y-2 pl-6", ulClass)} {...props} />
    ),
    ol: ({ className: olClass, ...props }: any) => (
      <ol className={cn("my-4 list-decimal space-y-2 pl-6", olClass)} {...props} />
    ),
    img: ({ src, alt, ...imgProps }: any) => {
      if (src) {
        const matchedAttachment = inlineAttachmentMap.get(src);
        if (matchedAttachment?.type === "video") {
          return (
            <VideoPreview
              src={src}
              mimeType={matchedAttachment.mime || "video/mp4"}
              description={matchedAttachment.description || alt}
              className="my-2 max-w-full"
              maxHeight="360px"
            />
          );
        }
      }

      return <MarkdownImage src={src} alt={alt} {...imgProps} />;
    },
  };

  const mergedComponents = {
    ...defaultComponents,
    ...components,
  };

  if (!components?.code) {
    mergedComponents.code = defaultCodeRenderer;
  }

  return (
    <div className={cn("markdown-body", containerClassName)}>
      <MarkdownPreview
        className={cn("markdown-body__content prose prose-sm max-w-none text-foreground", className)}
        source={content}
        remarkPlugins={[remarkGfm]}
        wrapperElement={{ "data-color-mode": "light" }}
        components={mergedComponents as any}
      />
    </div>
  );
}

export type MarkdownImageProps = React.ImgHTMLAttributes<HTMLImageElement>;

export function MarkdownImage({ className, alt, src, style, ...props }: MarkdownImageProps) {
  const [isPreviewOpen, setIsPreviewOpen] = useState(false);

  if (!src) {
    return null;
  }

  const altText = typeof alt === "string" ? alt : "";
  const thumbnailStyle = {
    ...(style || {}),
    maxWidth: style?.maxWidth ?? "min(100%, 360px)",
    height: "auto",
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setIsPreviewOpen(true)}
        className="my-2 mr-2 inline-flex max-w-full overflow-hidden rounded-2xl bg-transparent p-0 align-middle focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/60 cursor-zoom-in"
        aria-label={altText ? `查看 ${altText} 大图` : "查看大图"}
      >
        <img
          className={cn(
            "h-auto max-h-[360px] max-w-full object-contain transition-transform duration-300 hover:scale-[1.01]",
            className,
          )}
          alt={altText}
          src={src}
          style={thumbnailStyle}
          {...props}
        />
      </button>
      <Dialog open={isPreviewOpen} onOpenChange={setIsPreviewOpen}>
        <DialogContent
          className="bg-transparent p-0"
          onClose={() => setIsPreviewOpen(false)}
          showCloseButton={false}
          unstyled
        >
          <img
            className="h-auto max-h-[80vh] w-full max-w-[90vw] rounded-lg object-contain"
            alt={altText}
            src={src}
            {...props}
          />
        </DialogContent>
      </Dialog>
    </>
  );
}
