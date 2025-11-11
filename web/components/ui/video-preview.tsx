"use client";

import { type ComponentPropsWithoutRef } from "react";
import { cn } from "@/lib/utils";

type NativeVideoProps = Omit<
  ComponentPropsWithoutRef<"video">,
  "children" | "className"
>;

interface VideoPreviewProps extends NativeVideoProps {
  src: string;
  mimeType?: string;
  description?: string;
  className?: string;
  videoClassName?: string;
  minHeight?: string;
  maxHeight?: string;
}

export function VideoPreview({
  src,
  mimeType = "video/mp4",
  description,
  className,
  videoClassName,
  minHeight = "12rem",
  maxHeight = "20rem",
  controls = true,
  preload = "metadata",
  ...videoProps
}: VideoPreviewProps) {
  return (
    <div className={cn("w-full space-y-2", className)}>
      <div
        className="relative w-full overflow-hidden rounded-2xl bg-black"
        style={{ minHeight, maxHeight }}
      >
        <video
          {...videoProps}
          controls={controls}
          preload={preload}
          className={cn("h-full w-full object-contain bg-black", videoClassName)}
        >
          <source src={src} type={mimeType} />
          Your browser does not support video playback.
        </video>
      </div>
      {description && (
        <p className="text-[11px] text-muted-foreground">{description}</p>
      )}
    </div>
  );
}
