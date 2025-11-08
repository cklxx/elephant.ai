"use client";

import { useState } from "react";
import Image from "next/image";
import { Dialog, DialogContent } from "@/components/ui/dialog";
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
          "group relative block w-full overflow-hidden rounded-2xl cursor-zoom-in focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/60",
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
          className="bg-background/95 p-0"
          onClose={() => setOpen(false)}
          showCloseButton={false}
          unstyled
        >
          <div className="relative mx-auto w-full max-w-5xl">
            <div
              className="relative w-full"
              style={{ minHeight: "40vh", maxHeight: "85vh" }}
            >
              <Image
                src={src}
                alt={altText}
                fill
                className="object-contain"
                sizes="100vw"
                priority
                unoptimized
              />
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </>
  );
}
