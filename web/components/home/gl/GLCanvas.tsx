"use client";

import { Suspense, useEffect, useState } from "react";
import { Canvas } from "@react-three/fiber";
import { EffectComposer, Bloom } from "@react-three/postprocessing";
import { getGLCapabilities, type GLCapabilities } from "@/lib/webgl";
import { ParticleField } from "./ParticleField";

// ── CSS fallback for no-WebGL environments ──────────────────

function CSSFallback() {
  return (
    <div
      className="fixed inset-0"
      style={{
        background: "#080810",
      }}
    >
      {/* Animated gradient orbs as CSS-only alternative */}
      <div
        className="absolute inset-0 animate-[gradient_12s_ease-in-out_infinite]"
        style={{
          background: [
            "radial-gradient(600px circle at 20% 30%, rgba(52,211,153,0.15), transparent 60%)",
            "radial-gradient(800px circle at 80% 20%, rgba(45,212,191,0.10), transparent 65%)",
            "radial-gradient(500px circle at 50% 70%, rgba(52,211,153,0.08), transparent 55%)",
          ].join(", "),
        }}
      />
    </div>
  );
}

// ── Scene content ───────────────────────────────────────────

function Scene({ caps }: { caps: GLCapabilities }) {
  return (
    <>
      <ParticleField count={caps.particleCount} />
      {caps.bloom && (
        <EffectComposer>
          <Bloom
            luminanceThreshold={0.2}
            luminanceSmoothing={0.9}
            intensity={0.8}
            mipmapBlur
          />
        </EffectComposer>
      )}
    </>
  );
}

// ── Visibility hook: pause Canvas when tab is hidden ─────────

function usePageVisible() {
  const [visible, setVisible] = useState(true);
  useEffect(() => {
    const onChange = () => setVisible(document.visibilityState === "visible");
    document.addEventListener("visibilitychange", onChange);
    return () => document.removeEventListener("visibilitychange", onChange);
  }, []);
  return visible;
}

// ── Main GLCanvas component ─────────────────────────────────

export function GLCanvas() {
  const caps = getGLCapabilities();
  const visible = usePageVisible();

  if (!caps.webgl) {
    return <CSSFallback />;
  }

  return (
    <div className="fixed inset-0" style={{ background: "#080810" }}>
      <Canvas
        dpr={[1, caps.dpr]}
        camera={{ position: [0, 0, 30], fov: 60, near: 0.1, far: 100 }}
        gl={{
          alpha: false,
          antialias: false,
          powerPreference: "default",
        }}
        frameloop={visible ? "always" : "never"}
        style={{ background: "#080810" }}
      >
        <Suspense fallback={null}>
          <Scene caps={caps} />
        </Suspense>
      </Canvas>
    </div>
  );
}
