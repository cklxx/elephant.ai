"use client";

import { Download } from "lucide-react";
import {
  type ComponentPropsWithoutRef,
  useSyncExternalStore,
  useState,
} from "react";
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
  maxWidth?: string | number;
}

function subscribeCanHover(onStoreChange: () => void): () => void {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return () => {};
  }

  const mediaQuery = window.matchMedia("(hover: hover)");

  if (typeof mediaQuery.addEventListener === "function") {
    mediaQuery.addEventListener("change", onStoreChange);
    return () => mediaQuery.removeEventListener("change", onStoreChange);
  }

  mediaQuery.addListener(onStoreChange);
  return () => mediaQuery.removeListener(onStoreChange);
}

function getCanHoverSnapshot(): boolean {
  if (typeof window === "undefined" || typeof window.matchMedia !== "function") {
    return false;
  }
  return window.matchMedia("(hover: hover)").matches;
}

function getCanHoverServerSnapshot(): boolean {
  return false;
}

export function VideoPreview({
  src,
  mimeType = "video/mp4",
  description,
  className,
  videoClassName,
  maxHeight = "480px",
  maxWidth = "min(100%, 480px)",
  controls = false,
  preload = "metadata",
  ...videoProps
}: VideoPreviewProps) {
  const accessibleLabel = description
    ? `Video preview: ${description}`
    : undefined;
  const [isHovered, setIsHovered] = useState(false);
  const [isFocused, setIsFocused] = useState(false);
  const canHover = useSyncExternalStore(
    subscribeCanHover,
    getCanHoverSnapshot,
    getCanHoverServerSnapshot,
  );

  const showControls = controls || (canHover ? isHovered : true) || isFocused;
  const resolvedMaxHeight =
    typeof maxHeight === "number" ? `${maxHeight}px` : maxHeight;
  const resolvedMaxWidth =
    typeof maxWidth === "number" ? `${maxWidth}px` : maxWidth;

  return (
    <div
      className={cn(
        "relative inline-flex max-w-full overflow-hidden align-middle",
        className,
      )}
      style={{ maxHeight: resolvedMaxHeight, maxWidth: resolvedMaxWidth, width: resolvedMaxWidth }}
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
          "block h-auto w-full max-h-full object-contain object-center",
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
          className="absolute bottom-2 right-2 inline-flex items-center gap-1 px-2.5 py-1 text-xs"
          aria-label={description ? `下载 ${description}` : "下载视频"}
        >
          <Download className="h-4 w-4" aria-hidden="true" />
          <span className="hidden sm:inline">下载</span>
        </a>
      ) : null}
    </div>
  );
}
