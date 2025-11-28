"use client";

import { Download } from "lucide-react";
import { type ComponentPropsWithoutRef, useEffect, useState } from "react";
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
  maxHeight?: string | number;
}

export function VideoPreview({
  src,
  mimeType = "video/mp4",
  description,
  className,
  videoClassName,
  maxHeight = "480px",
  controls = false,
  preload = "metadata",
  ...videoProps
}: VideoPreviewProps) {
  const accessibleLabel = description
    ? `Video preview: ${description}`
    : undefined;
  const [isHovered, setIsHovered] = useState(false);
  const [isFocused, setIsFocused] = useState(false);
  const [canHover, setCanHover] = useState(false);

  useEffect(() => {
    if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
      return;
    }

    const mediaQuery = window.matchMedia("(hover: hover)");
    const handleChange = (event: MediaQueryListEvent) => {
      setCanHover(event.matches);
    };

    setCanHover(mediaQuery.matches);

    if (typeof mediaQuery.addEventListener === "function") {
      mediaQuery.addEventListener("change", handleChange);
      return () => mediaQuery.removeEventListener("change", handleChange);
    }

    mediaQuery.addListener(handleChange);
    return () => mediaQuery.removeListener(handleChange);
  }, []);

  const showControls = controls || (canHover ? isHovered : true) || isFocused;
  const resolvedMaxHeight =
    typeof maxHeight === "number" ? `${maxHeight}px` : maxHeight;

  return (
    <div
      className={cn(
        "relative inline-flex max-w-full overflow-hidden rounded-2xl bg-black align-middle",
        className,
      )}
      style={{ maxHeight: resolvedMaxHeight }}
      onMouseEnter={() => setIsHovered(true)}
      onMouseLeave={() => setIsHovered(false)}
    >
      <video
        {...videoProps}
        controls={showControls}
        preload={preload}
        aria-label={accessibleLabel}
        title={description}
        className={cn(
          "block h-auto w-full max-h-full object-contain object-center bg-black",
          videoClassName,
        )}
        onFocus={() => setIsFocused(true)}
        onBlur={() => setIsFocused(false)}
      >
        <source src={src} type={mimeType} />
        Your browser does not support video playback.
      </video>
      {src ? (
        <a
          href={src}
          download
          className="absolute bottom-2 right-2 inline-flex items-center gap-1 rounded-full bg-black/70 px-2.5 py-1 text-xs font-medium text-white shadow transition hover:bg-black/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-black"
          aria-label={description ? `下载 ${description}` : "下载视频"}
        >
          <Download className="h-4 w-4" aria-hidden="true" />
          <span className="hidden sm:inline">下载</span>
        </a>
      ) : null}
    </div>
  );
}
