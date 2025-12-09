/* eslint-disable @next/next/no-img-element */
"use client";

import { useState } from "react";
import Image from "next/image";
import { Dialog, DialogContent, DialogTitle } from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

interface ImagePreviewProps {
  src: string;
  alt?: string;
  className?: string;
  imageClassName?: string;
  sizes?: string;
  minHeight?: string;
  maxHeight?: string;
}

export function ImagePreview({
  src,
  alt,
  className,
  imageClassName,
  sizes = "100vw",
  minHeight = "12rem",
  maxHeight = "20rem",
}: ImagePreviewProps) {
  const [open, setOpen] = useState(false);
  const altText = alt?.trim() || "Image preview";
  const label = alt ? `查看 ${alt} 大图` : "查看大图";

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={label}
        className={cn(
          "group relative block w-full overflow-hidden cursor-zoom-in",
          className,
        )}
      >
        <div
          className="relative w-full"
          style={{ minHeight, maxHeight }}
        >
          <Image
            src={src}
            alt={altText}
            fill
            className={cn("object-contain transition-transform duration-300 group-hover:scale-[1.02]", imageClassName)}
            sizes={sizes}
            unoptimized
          />
        </div>
      </button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent
          className="p-0 max-w-none w-screen sm:w-[90vw]"
          onClose={() => setOpen(false)}
          showCloseButton={false}
          unstyled
        >
          <DialogTitle className="sr-only">{altText}</DialogTitle>
          <div className="flex max-h-[85vh] w-full items-center justify-center px-4 py-6">
            <img
              src={src}
              alt={altText}
              className="h-auto max-h-[80vh] w-auto max-w-full object-contain"
            />
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
