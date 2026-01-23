import { Children, ComponentType, ReactNode, isValidElement, useMemo } from "react";

import { VideoPreview } from "@/components/ui/video-preview";
import { cn } from "@/lib/utils";

import { MarkdownImage } from "../components/MarkdownImage";
import {
  MarkdownTable,
  MarkdownTableBody,
  MarkdownTableCell,
  MarkdownTableHead,
  MarkdownTableHeaderCell,
  MarkdownTableRow,
} from "../components/MarkdownTable";
import { createMarkdownCodeRenderer } from "../components/MarkdownCode";

interface InlineAttachmentInfo {
  type: string;
  description?: string;
  mime?: string;
}

interface UseMarkdownComponentsOptions {
  showLineNumbers: boolean;
  inlineAttachmentMap: Map<string, InlineAttachmentInfo>;
  components?: Record<string, ComponentType<any>>;
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

export function useMarkdownComponents({
  showLineNumbers,
  inlineAttachmentMap,
  components,
}: UseMarkdownComponentsOptions) {
  const defaultComponents: Record<string, ComponentType<any>> = useMemo(
    () => ({
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
        <strong className={cn("font-medium text-foreground", strongClass)} {...props} />
      ),
      p: ({ className: paragraphClass, ...props }: any) => (
        <p className={cn("my-0 whitespace-pre-wrap", paragraphClass)} {...props} />
      ),
      hr: (props: any) => <hr className={cn("my-6", props.className)} {...props} />,
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
      table: MarkdownTable,
      thead: MarkdownTableHead,
      tbody: MarkdownTableBody,
      tr: MarkdownTableRow,
      th: MarkdownTableHeaderCell,
      td: MarkdownTableCell,
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
    }),
    [inlineAttachmentMap],
  );

  const mergedComponents = useMemo(() => {
    const merged = {
      ...defaultComponents,
      ...components,
    };
    if (!components?.code) {
      merged.code = createMarkdownCodeRenderer(showLineNumbers);
    }
    return merged;
  }, [components, defaultComponents, showLineNumbers]);

  return mergedComponents;
}
