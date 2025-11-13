"use client";

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
  maxHeight?: string;
}

export function VideoPreview({
  src,
  mimeType = "video/mp4",
  description,
  className,
  videoClassName,
  maxHeight = "20rem",
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

  return (
    <div
      className={cn(
        "self-center relative w-full overflow-hidden rounded-2xl bg-black",
        className,
      )}
      style={{ maxHeight }}
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
          "block h-full w-full object-cover object-center bg-black",
          videoClassName,
        )}
        onFocus={() => setIsFocused(true)}
        onBlur={() => setIsFocused(false)}
      >
        <source src={src} type={mimeType} />
        Your browser does not support video playback.
      </video>
    </div>
  );
}
