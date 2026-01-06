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
  loading?: "eager" | "lazy";
}

export function ImagePreview({
  src,
  alt,
  className,
  imageClassName,
  sizes = "100vw",
  minHeight = "12rem",
  maxHeight = "20rem",
  loading = "lazy",
}: ImagePreviewProps) {
  const [open, setOpen] = useState(false);
  const [isLoaded, setIsLoaded] = useState(false);
  const [hasError, setHasError] = useState(false);
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
          <div
            className={cn(
              "absolute inset-0 rounded-md bg-muted/30",
              !isLoaded && !hasError && "animate-pulse",
              (isLoaded || hasError) && "opacity-0",
            )}
            aria-hidden="true"
          />
          {hasError ? (
            <div className="absolute inset-0 flex items-center justify-center text-xs text-muted-foreground">
              Image unavailable
            </div>
          ) : (
            <Image
              src={src}
              alt={altText}
              fill
              className={cn(
                "object-contain transition-opacity duration-300 group-hover:scale-[1.02]",
                isLoaded ? "opacity-100" : "opacity-0",
                imageClassName,
              )}
              sizes={sizes}
              unoptimized
              loading={loading}
              priority={loading === "eager"}
              onLoadingComplete={() => setIsLoaded(true)}
              onError={() => setHasError(true)}
            />
          )}
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
