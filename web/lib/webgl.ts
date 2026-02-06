/**
 * WebGL capability detection and device classification.
 *
 * Used by the GL homepage to decide particle count, bloom,
 * and whether to fall back to a CSS-only background.
 */

export type DeviceTier = "high" | "low" | "none";

export interface GLCapabilities {
  /** Whether WebGL 2 (or at least WebGL 1) is available */
  webgl: boolean;
  /** Device performance tier */
  tier: DeviceTier;
  /** Recommended particle count */
  particleCount: number;
  /** Whether to enable bloom postprocessing */
  bloom: boolean;
  /** Device pixel ratio cap */
  dpr: number;
}

/** Detect WebGL support by attempting to create a context. */
export function detectWebGL(): boolean {
  if (typeof document === "undefined") return false;
  try {
    const canvas = document.createElement("canvas");
    const gl =
      canvas.getContext("webgl2") || canvas.getContext("webgl");
    return gl !== null;
  } catch {
    return false;
  }
}

/** Simple mobile heuristic via user-agent and touch points. */
export function isMobileDevice(): boolean {
  if (typeof navigator === "undefined") return false;
  const ua = navigator.userAgent;
  if (/Android|iPhone|iPad|iPod|webOS|BlackBerry|Opera Mini/i.test(ua)) {
    return true;
  }
  return navigator.maxTouchPoints > 1 && window.innerWidth < 1024;
}

/** Classify the device and return recommended GL settings. */
export function getGLCapabilities(): GLCapabilities {
  const webgl = detectWebGL();
  if (!webgl) {
    return { webgl: false, tier: "none", particleCount: 0, bloom: false, dpr: 1 };
  }

  const mobile = isMobileDevice();
  const rawDpr = typeof window !== "undefined" ? window.devicePixelRatio : 1;
  const dpr = Math.min(rawDpr, 2);

  if (mobile) {
    return { webgl: true, tier: "low", particleCount: 200, bloom: false, dpr };
  }

  return { webgl: true, tier: "high", particleCount: 500, bloom: true, dpr };
}
