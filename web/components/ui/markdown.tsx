"use client";
/* eslint-disable @next/next/no-img-element */

import { Streamdown } from "streamdown";
import { Highlight, Language, themes } from "prism-react-renderer";
import { cn } from "@/lib/utils";
import {
  Children,
  ComponentType,
  ReactNode,
  isValidElement,
  useMemo,
  useState,
} from "react";
import { VideoPreview } from "@/components/ui/video-preview";
import {
  buildAttachmentUri,
  getAttachmentSegmentType,
} from "@/lib/attachments";
import { AttachmentPayload } from "@/lib/types";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";

export type MarkdownRendererProps = {
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
  mode?: "static" | "streaming";
  components?: Record<string, ComponentType<any>>;
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

  const defaultCodeRenderer = ({ className, children, ...props }: any) => {
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

  const splitTaskListChildren = (children: ReactNode) => {
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
  };

  const defaultComponents: Record<string, ComponentType<any>> = {
    h1: ({ className: headingClass, ...props }: any) => (
      <h2
        className={cn(
          "mt-3 mb-2 scroll-m-20 text-base font-medium leading-snug tracking-tight",
          headingClass,
        )}
        {...props}
      />
    ),
    h2: ({ className: headingClass, ...props }: any) => (
      <h3
        className={cn(
          "mt-2 mb-2 scroll-m-20 text-base font-medium leading-snug tracking-tight",
          headingClass,
        )}
        {...props}
      />
    ),
    h3: ({ className: headingClass, ...props }: any) => (
      <h4
        className={cn(
          "mt-2 mb-1.5 scroll-m-20 text-base font-medium leading-snug tracking-tight",
          headingClass,
        )}
        {...props}
      />
    ),
    h4: ({ className: headingClass, ...props }: any) => (
      <h5
        className={cn(
          "mt-3 mb-1 scroll-m-20 text-base font-medium leading-snug text-foreground/90",
          headingClass,
        )}
        {...props}
      />
    ),
    h5: ({ className: headingClass, ...props }: any) => (
      <h6
        className={cn(
          "mt-2 mb-1 scroll-m-20 text-base font-medium leading-snug text-muted-foreground",
          headingClass,
        )}
        {...props}
      />
    ),
    h6: ({ className: headingClass, ...props }: any) => (
      <h6
        className={cn(
          "mt-2 mb-1 scroll-m-20 text-base font-medium leading-snug text-muted-foreground",
          headingClass,
        )}
        {...props}
      />
    ),
    strong: ({ className: strongClass, ...props }: any) => (
      <strong
        className={cn("font-medium text-foreground", strongClass)}
        {...props}
      />
    ),
    p: ({ className: paragraphClass, ...props }: any) => (
      <p
        className={cn("my-0 whitespace-pre-wrap", paragraphClass)}
        {...props}
      />
    ),
    hr: (props: any) => (
      <hr className={cn("my-6", props.className)} {...props} />
    ),
    a: ({ className: linkClassName, href, children, ...props }: any) => {
      const matchedAttachment = href
        ? inlineAttachmentMap.get(href)
        : undefined;

      if (matchedAttachment?.type === "video") {
        return (
          <VideoPreview
            src={href}
            mimeType={matchedAttachment.mime || "video/mp4"}
            description={
              matchedAttachment.description ||
              (typeof children === "string" ? children : undefined)
            }
            className="my-2 max-w-full"
            maxHeight="320px"
            maxWidth="480px"
          />
        );
      }

      return (
        <a
          className={cn("break-words whitespace-normal", linkClassName)}
          href={href}
          {...props}
        >
          {children}
        </a>
      );
    },
    blockquote: ({ className: blockquoteClass, ...props }: any) => (
      <blockquote className={cn(blockquoteClass)} {...props} />
    ),
    table: ({ className: tableClass, ...props }: any) => (
      <div className="my-4 overflow-x-auto">
        <table className={cn("w-full", tableClass)} {...props} />
      </div>
    ),
    th: ({ className: thClass, ...props }: any) => (
      <th className={cn("px-4 py-2 text-left", thClass)} {...props} />
    ),
    td: ({ className: tdClass, ...props }: any) => (
      <td className={cn("px-4 py-2 align-top", tdClass)} {...props} />
    ),
    ul: ({ className: ulClass, ...props }: any) => {
      const isTaskList =
        typeof ulClass === "string" && ulClass.includes("contains-task-list");
      return (
        <ul
          className={cn(
            "my-4 !pl-0 flex flex-col gap-2",
            isTaskList ? "list-none" : "list-disc list-inside",
            ulClass,
          )}
          {...props}
        />
      );
    },
    ol: ({ className: olClass, ...props }: any) => {
      const isTaskList =
        typeof olClass === "string" && olClass.includes("contains-task-list");
      return (
        <ol
          className={cn(
            "my-4 !pl-0 flex flex-col gap-2",
            isTaskList ? "list-none" : "list-decimal list-inside",
            olClass,
          )}
          {...props}
        />
      );
    },
    li: ({ className: liClass, children, ...props }: any) => {
      const taskChildren = splitTaskListChildren(children);
      if (taskChildren) {
        return (
          <li className={cn("flex items-start gap-2", liClass)} {...props}>
            {taskChildren.checkbox}
            <div className="min-w-0 flex-1">{taskChildren.rest}</div>
          </li>
        );
      }
      return (
        <li className={cn("", liClass)} {...props}>
          {children}
        </li>
      );
    },
    input: ({ className: inputClass, type, ...props }: any) => {
      if (type === "checkbox") {
        return (
          <input
            type="checkbox"
            className={cn(
              "mt-1 h-4 w-4 shrink-0 accent-foreground",
              inputClass,
            )}
            {...props}
          />
        );
      }
      return <input type={type} className={cn(inputClass)} {...props} />;
    },
    br: () => null,
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
              maxHeight="320px"
              maxWidth="480px"
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
      <Streamdown
        mode={mode}
        className={cn(
          "wmde-markdown markdown-body__content prose prose-sm max-w-none text-foreground prose-a:break-words prose-a:whitespace-normal",
          className,
        )}
        components={mergedComponents as any}
      >
        {content}
      </Streamdown>
    </div>
  );
}

export type MarkdownImageProps = React.ImgHTMLAttributes<HTMLImageElement>;

export function MarkdownImage({
  className,
  alt,
  src,
  style,
  ...props
}: MarkdownImageProps) {
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
          <DialogTitle className="sr-only">
            {altText || "Image preview"}
          </DialogTitle>
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
