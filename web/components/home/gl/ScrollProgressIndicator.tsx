"use client";

import { useRef } from "react";
import { useFrame } from "@react-three/fiber";
import { useScroll } from "@react-three/drei";
import { MathUtils } from "three";

export function ScrollProgressIndicator() {
  const trackRef = useRef<HTMLDivElement>(null);
  const thumbRef = useRef<HTMLDivElement>(null);
  const scroll = useScroll();
  const smoothOffset = useRef(0);

  useFrame((_, delta) => {
    if (!trackRef.current || !thumbRef.current) return;
    smoothOffset.current = MathUtils.damp(smoothOffset.current, scroll.offset, 6, delta);

    const pct = smoothOffset.current * 100;
    thumbRef.current.style.top = `${pct}%`;

    // Fade the track in/out at extremes
    const opacity = smoothOffset.current < 0.02 ? smoothOffset.current / 0.02 : 1;
    trackRef.current.style.opacity = String(opacity);
  });

  return (
    <div
      ref={trackRef}
      className="fixed right-6 top-1/2 z-20 -translate-y-1/2"
      style={{
        width: 2,
        height: "30vh",
        background: "rgba(255,255,255,0.06)",
        borderRadius: 1,
      }}
    >
      <div
        ref={thumbRef}
        className="absolute left-1/2 -translate-x-1/2"
        style={{
          width: 8,
          height: 8,
          borderRadius: "50%",
          background: "#818cf8",
          boxShadow: "0 0 12px rgba(129,140,248,0.5)",
          marginTop: -4,
        }}
      />
    </div>
  );
}
