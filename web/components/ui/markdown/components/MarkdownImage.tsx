/* eslint-disable @next/next/no-img-element */

import { useState } from "react";

import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

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
